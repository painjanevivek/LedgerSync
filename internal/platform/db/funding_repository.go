package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/ledger"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

type FundingRepository struct {
	database *sql.DB
	clock    func() time.Time
}

func NewFundingRepository(database *sql.DB, clock func() time.Time) (*FundingRepository, error) {
	if database == nil {
		return nil, errors.New("funding database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &FundingRepository{database: database, clock: clock}, nil
}

type fundingPolicy struct {
	Mode            string
	FinanceActive   bool
	Version         int64
	PerCommand      int64
	OperatorRolling int64
	TenantRolling   int64
}

type fundingRowScanner interface{ Scan(...any) error }

const fundingEventColumns = `
id::text,status,destination_account_id::text,system_account_id::text,currency,amount_minor,
external_reference,evidence_reference,requester_subject_id,COALESCE(approver_subject_id,''),
COALESCE(decision_reason,''),demo_policy,COALESCE(journal_transaction_id::text,''),
COALESCE(compensation_of_event_id::text,''),COALESCE(compensation_event_id::text,''),
COALESCE(compensation_reason_code,''),COALESCE(compensation_operator_note,''),requested_at,updated_at,
CASE WHEN status IN ('posted','compensated') THEN COALESCE((SELECT balance_version::text FROM account_balance_projections WHERE account_id=destination_account_id),'') ELSE '' END`

func (r *FundingRepository) Request(ctx context.Context, command appfunding.RequestCommand, fingerprint [sha256.Size]byte) (submission appfunding.Submission, err error) {
	sequence := "funding-request|" + strings.ToLower(command.TenantID) + "|" + command.ActorSubjectID
	err = WithSerializableSequence(ctx, r.database, sequence, 5, func(tx *sql.Tx) error {
		if err := authorizeFinanceActor(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		var existingFingerprint []byte
		existingErr := tx.QueryRowContext(ctx, `SELECT request_fingerprint FROM funding_events WHERE tenant_id=$1 AND requester_subject_id=$2 AND idempotency_key=$3`, command.TenantID, command.ActorSubjectID, command.IdempotencyKey).Scan(&existingFingerprint)
		if existingErr == nil {
			if string(existingFingerprint) != string(fingerprint[:]) {
				return appfunding.ErrConflict
			}
			event, err := readFundingEvent(ctx, tx, command.TenantID, command.IdempotencyKey, command.ActorSubjectID)
			if err != nil {
				return err
			}
			submission = appfunding.Submission{Event: event, Replayed: true}
			return nil
		}
		if !errors.Is(existingErr, sql.ErrNoRows) {
			return existingErr
		}

		currency := command.Amount.Currency().Code
		policy, err := loadFundingPolicy(ctx, tx, command.TenantID, command.DestinationAccountID, currency)
		if err != nil {
			return err
		}
		if policy.Mode == string(appfunding.PolicyProductionDualControl) && !policy.FinanceActive {
			return appfunding.ErrForbidden
		}
		if command.Amount.Minor() > policy.PerCommand {
			return appfunding.ErrLimitExceeded
		}
		systemAccountID, err := ensureFundingAccount(ctx, tx, command.TenantID, currency, command.RequestedAt)
		if err != nil {
			return err
		}
		eventID, err := newUUID()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO funding_events(
 id,tenant_id,requester_subject_id,destination_account_id,system_account_id,
 external_reference,evidence_reference,idempotency_key,request_fingerprint,
 amount_minor,currency,status,demo_policy,policy_version,correlation_id,requested_at,updated_at
) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'requested',false,$12,$13,$14,$14)`,
			eventID, command.TenantID, command.ActorSubjectID, command.DestinationAccountID, systemAccountID,
			command.ExternalReference, command.EvidenceReference, command.IdempotencyKey, fingerprint[:], command.Amount.Minor(), currency,
			policy.Version, command.CorrelationID, command.RequestedAt)
		if err != nil {
			return classifyFundingInsertError("insert funding request", err)
		}
		approvalID, err := newUUID()
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `
INSERT INTO approval_records(id,tenant_id,command_type,target_id,requester_subject_id,status,expires_at,correlation_id,policy_version,created_at)
VALUES($1,$2,'funding',$3,$4,'requested',$5,$6,$7,$8)`, approvalID, command.TenantID, eventID, command.ActorSubjectID, command.RequestedAt.Add(24*time.Hour), command.CorrelationID, policy.Version, command.RequestedAt); err != nil {
			return fmt.Errorf("insert funding approval request: %w", err)
		}
		if err = insertFundingAudit(ctx, tx, command.TenantID, command.ActorSubjectID, eventID, "funding.requested", "succeeded", command.CorrelationID, command.RequestedAt); err != nil {
			return err
		}
		event, err := readFundingEventByID(ctx, tx, command.TenantID, eventID)
		if err != nil {
			return err
		}
		submission = appfunding.Submission{Event: event}
		return nil
	})
	return submission, err
}

func (r *FundingRepository) Approve(ctx context.Context, command appfunding.DecisionCommand, demo bool) (event appfunding.Event, err error) {
	err = WithSerializableSequence(ctx, r.database, "funding-decision|"+strings.ToLower(command.TenantID)+"|"+command.FundingEventID, 5, func(tx *sql.Tx) error {
		if err := authorizeFinanceActor(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		var requester, status, mode, compensationOf string
		var financeActive bool
		var policyVersion int64
		err := tx.QueryRowContext(ctx, `
SELECT event.requester_subject_id,event.status,policy.mode,policy.finance_activated,policy.policy_version,
 COALESCE(event.compensation_of_event_id::text,'')
FROM funding_events event JOIN tenant_funding_policies policy ON policy.tenant_id=event.tenant_id AND policy.currency=event.currency
WHERE event.tenant_id=$1 AND event.id=$2 FOR UPDATE OF event`, command.TenantID, command.FundingEventID).Scan(&requester, &status, &mode, &financeActive, &policyVersion, &compensationOf)
		if errors.Is(err, sql.ErrNoRows) {
			return appfunding.ErrNotFound
		}
		if err != nil {
			return err
		}
		localDemo := mode == string(appfunding.PolicyLocalDemoSingleOperator)
		if localDemo != demo || (!localDemo && !financeActive) || (!localDemo && requester == command.ActorSubjectID) {
			return appfunding.ErrForbidden
		}
		if status == "approved" {
			event, err = readFundingEventByID(ctx, tx, command.TenantID, command.FundingEventID)
			return err
		}
		if status != "requested" {
			return appfunding.ErrConflict
		}
		result, err := tx.ExecContext(ctx, `
UPDATE funding_events SET status='approved',approver_subject_id=$3,decision_reason=$4,demo_policy=$5,approved_at=$6,updated_at=$6
WHERE tenant_id=$1 AND id=$2 AND status='requested'`, command.TenantID, command.FundingEventID, command.ActorSubjectID, command.Reason, localDemo, command.DecidedAt)
		if err != nil {
			return err
		}
		if err = requireOneRow(result, "approve funding"); err != nil {
			return appfunding.ErrConflict
		}
		approvalID, err := newUUID()
		if err != nil {
			return err
		}
		commandType := fundingCommandType(compensationOf)
		if _, err = tx.ExecContext(ctx, `
INSERT INTO approval_records(id,tenant_id,command_type,target_id,requester_subject_id,approver_subject_id,status,expires_at,decision_reason,correlation_id,policy_version,created_at,decided_at)
VALUES($1,$2,$3,$4,$5,$6,'approved',$7,$8,$9,$10,$11,$11)`, approvalID, command.TenantID, commandType, command.FundingEventID, requester, command.ActorSubjectID, command.DecidedAt.Add(24*time.Hour), command.Reason, command.CorrelationID, policyVersion, command.DecidedAt); err != nil {
			return err
		}
		if err = insertFundingAudit(ctx, tx, command.TenantID, command.ActorSubjectID, command.FundingEventID, "funding.approved", "succeeded", command.CorrelationID, command.DecidedAt); err != nil {
			return err
		}
		event, err = readFundingEventByID(ctx, tx, command.TenantID, command.FundingEventID)
		return err
	})
	return event, err
}

func (r *FundingRepository) Reject(ctx context.Context, command appfunding.DecisionCommand) (event appfunding.Event, err error) {
	err = WithSerializableSequence(ctx, r.database, "funding-decision|"+strings.ToLower(command.TenantID)+"|"+command.FundingEventID, 5, func(tx *sql.Tx) error {
		if err := authorizeFinanceActor(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		var requester, compensationOf string
		var policyVersion int64
		err := tx.QueryRowContext(ctx, `SELECT requester_subject_id,policy_version,COALESCE(compensation_of_event_id::text,'') FROM funding_events WHERE tenant_id=$1 AND id=$2 AND status='requested' FOR UPDATE`, command.TenantID, command.FundingEventID).Scan(&requester, &policyVersion, &compensationOf)
		if errors.Is(err, sql.ErrNoRows) {
			return appfunding.ErrConflict
		}
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE funding_events SET status='rejected',approver_subject_id=$3,decision_reason=$4,rejected_at=$5,updated_at=$5 WHERE tenant_id=$1 AND id=$2`, command.TenantID, command.FundingEventID, command.ActorSubjectID, command.Reason, command.DecidedAt); err != nil {
			return err
		}
		approvalID, _ := newUUID()
		if _, err = tx.ExecContext(ctx, `INSERT INTO approval_records(id,tenant_id,command_type,target_id,requester_subject_id,approver_subject_id,status,expires_at,decision_reason,correlation_id,policy_version,created_at,decided_at) VALUES($1,$2,$3,$4,$5,$6,'rejected',$7,$8,$9,$10,$11,$11)`, approvalID, command.TenantID, fundingCommandType(compensationOf), command.FundingEventID, requester, command.ActorSubjectID, command.DecidedAt, command.Reason, command.CorrelationID, policyVersion, command.DecidedAt); err != nil {
			return err
		}
		if err = insertFundingAudit(ctx, tx, command.TenantID, command.ActorSubjectID, command.FundingEventID, "funding.rejected", "succeeded", command.CorrelationID, command.DecidedAt); err != nil {
			return err
		}
		event, err = readFundingEventByID(ctx, tx, command.TenantID, command.FundingEventID)
		return err
	})
	return event, err
}

func (r *FundingRepository) Post(ctx context.Context, command appfunding.ActionCommand) (submission appfunding.Submission, err error) {
	err = WithSerializableSequence(ctx, r.database, "funding-post|"+strings.ToLower(command.TenantID), 5, func(tx *sql.Tx) error {
		if err := authorizeFinanceActor(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		var requester, destinationID, systemID, currency, status, compensationOf, storedPostKey string
		var amount int64
		var policy fundingPolicy
		err := tx.QueryRowContext(ctx, `
SELECT event.requester_subject_id,event.destination_account_id::text,event.system_account_id::text,event.currency,event.amount_minor,event.status,
 policy.mode,policy.finance_activated,policy.per_command_minor,policy.operator_rolling_24h_minor,policy.tenant_rolling_24h_minor,
 COALESCE(event.compensation_of_event_id::text,''),COALESCE(event.post_idempotency_key,'')
FROM funding_events event JOIN tenant_funding_policies policy ON policy.tenant_id=event.tenant_id AND policy.currency=event.currency
WHERE event.tenant_id=$1 AND event.id=$2 FOR UPDATE OF event`, command.TenantID, command.FundingEventID).Scan(&requester, &destinationID, &systemID, &currency, &amount, &status, &policy.Mode, &policy.FinanceActive, &policy.PerCommand, &policy.OperatorRolling, &policy.TenantRolling, &compensationOf, &storedPostKey)
		if errors.Is(err, sql.ErrNoRows) {
			return appfunding.ErrNotFound
		}
		if err != nil {
			return err
		}
		if status == "posted" || status == "compensated" {
			if storedPostKey != "" && storedPostKey != command.IdempotencyKey {
				return appfunding.ErrConflict
			}
			event, readErr := readFundingEventByID(ctx, tx, command.TenantID, command.FundingEventID)
			if readErr != nil {
				return readErr
			}
			submission = appfunding.Submission{Event: event, Replayed: true}
			return nil
		}
		if status != "approved" || (policy.Mode == string(appfunding.PolicyProductionDualControl) && !policy.FinanceActive) {
			return appfunding.ErrForbidden
		}
		var approvalValid bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM approval_records WHERE tenant_id=$1 AND target_id=$2 AND command_type=$3 AND status='approved' AND expires_at>$4)`, command.TenantID, command.FundingEventID, fundingCommandType(compensationOf), command.OccurredAt).Scan(&approvalValid); err != nil || !approvalValid {
			return appfunding.ErrForbidden
		}
		if compensationOf == "" {
			var actorRolling, tenantRolling int64
			if err = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(amount_minor) FILTER (WHERE actor_subject_id=$2),0),COALESCE(sum(amount_minor),0) FROM funding_velocity_events WHERE tenant_id=$1 AND expires_at>$3`, command.TenantID, requester, command.OccurredAt).Scan(&actorRolling, &tenantRolling); err != nil {
				return err
			}
			if amount > policy.PerCommand || actorRolling > policy.OperatorRolling-amount || tenantRolling > policy.TenantRolling-amount {
				return appfunding.ErrLimitExceeded
			}
		}
		journalID, _ := newUUID()
		if _, err = tx.ExecContext(ctx, `INSERT INTO journal_transactions(id,tenant_id,funding_event_id,occurred_at) VALUES($1,$2,$3,$4)`, journalID, command.TenantID, command.FundingEventID, command.OccurredAt); err != nil {
			return err
		}
		exact, err := money.New(currency, amount)
		if err != nil {
			return err
		}
		debitID, _ := newUUID()
		creditID, _ := newUUID()
		debitAccountID, creditAccountID := systemID, destinationID
		if compensationOf != "" {
			debitAccountID, creditAccountID = destinationID, systemID
		}
		debit, err := ledger.NewPosting(debitID, journalID, debitAccountID, ledger.Debit, exact, command.OccurredAt)
		if err != nil {
			return err
		}
		credit, err := ledger.NewPosting(creditID, journalID, creditAccountID, ledger.Credit, exact, command.OccurredAt)
		if err != nil {
			return err
		}
		if err = ledger.ValidateBalanced([]ledger.Posting{debit, credit}); err != nil {
			return err
		}
		if err = createPosting(ctx, tx, debit); err != nil {
			return err
		}
		if err = createPosting(ctx, tx, credit); err != nil {
			return err
		}
		systemDelta, destinationDelta := -amount, amount
		if compensationOf != "" {
			systemDelta, destinationDelta = amount, -amount
		}
		result, err := tx.ExecContext(ctx, `UPDATE account_balance_projections SET available_minor=available_minor+$2,ledger_minor=ledger_minor+$2,balance_version=balance_version+1,updated_at=$3 WHERE account_id=$1 AND allow_negative=true`, systemID, systemDelta, command.OccurredAt)
		if err != nil {
			return err
		}
		if err = requireOneRow(result, "update funding clearing balance"); err != nil {
			return appfunding.ErrConflict
		}
		destination, err := applyBalanceDelta(ctx, tx, destinationID, destinationDelta, command.OccurredAt)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE funding_events SET status='posted',journal_transaction_id=$3,posted_at=$4,updated_at=$4,post_idempotency_key=$5 WHERE tenant_id=$1 AND id=$2 AND status='approved'`, command.TenantID, command.FundingEventID, journalID, command.OccurredAt, command.IdempotencyKey); err != nil {
			return err
		}
		if compensationOf == "" {
			if _, err = tx.ExecContext(ctx, `INSERT INTO funding_velocity_events(funding_event_id,tenant_id,actor_subject_id,amount_minor,occurred_at,expires_at) VALUES($1,$2,$3,$4,$5,$5::timestamptz+INTERVAL '24 hours')`, command.FundingEventID, command.TenantID, requester, amount, command.OccurredAt); err != nil {
				return err
			}
		} else {
			result, err = tx.ExecContext(ctx, `UPDATE funding_events SET status='compensated',compensation_event_id=$3,compensated_at=$4,updated_at=$4 WHERE tenant_id=$1 AND id=$2 AND status='posted'`, command.TenantID, compensationOf, command.FundingEventID, command.OccurredAt)
			if err != nil {
				return err
			}
			if err = requireOneRow(result, "link funding compensation"); err != nil {
				return appfunding.ErrConflict
			}
		}
		auditType := "funding.posted"
		if compensationOf != "" {
			auditType = "funding.compensation.posted"
		}
		if err = insertFundingAudit(ctx, tx, command.TenantID, command.ActorSubjectID, command.FundingEventID, auditType, "succeeded", command.CorrelationID, command.OccurredAt); err != nil {
			return err
		}
		if err = enqueueFundingBalanceEvent(ctx, tx, command, currency, destinationDelta, destination, command.OccurredAt); err != nil {
			return err
		}
		event, err := readFundingEventByID(ctx, tx, command.TenantID, command.FundingEventID)
		if err != nil {
			return err
		}
		event.BalanceVersion = strconv.FormatInt(destination.BalanceVersion, 10)
		submission = appfunding.Submission{Event: event}
		return nil
	})
	return submission, err
}

func (r *FundingRepository) Compensate(ctx context.Context, command appfunding.CompensationCommand, fingerprint [sha256.Size]byte) (submission appfunding.Submission, err error) {
	sequence := "funding-compensate|" + strings.ToLower(command.TenantID) + "|" + command.FundingEventID
	err = WithSerializableSequence(ctx, r.database, sequence, 5, func(tx *sql.Tx) error {
		if err := authorizeFinanceActor(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		var existingFingerprint []byte
		existingErr := tx.QueryRowContext(ctx, `SELECT request_fingerprint FROM funding_events WHERE tenant_id=$1 AND requester_subject_id=$2 AND idempotency_key=$3`, command.TenantID, command.ActorSubjectID, command.IdempotencyKey).Scan(&existingFingerprint)
		if existingErr == nil {
			if string(existingFingerprint) != string(fingerprint[:]) {
				return appfunding.ErrConflict
			}
			event, readErr := readFundingEvent(ctx, tx, command.TenantID, command.IdempotencyKey, command.ActorSubjectID)
			if readErr != nil {
				return readErr
			}
			submission = appfunding.Submission{Event: event, Replayed: true}
			return nil
		}
		if !errors.Is(existingErr, sql.ErrNoRows) {
			return existingErr
		}

		var destinationID, systemID, currency, evidenceReference, status, existingCompensation string
		var amount, policyVersion int64
		err := tx.QueryRowContext(ctx, `
SELECT destination_account_id::text,system_account_id::text,currency,amount_minor,evidence_reference,status,
 COALESCE(compensation_event_id::text,''),policy_version
FROM funding_events
WHERE tenant_id=$1 AND id=$2 AND compensation_of_event_id IS NULL
FOR UPDATE`, command.TenantID, command.FundingEventID).Scan(&destinationID, &systemID, &currency, &amount, &evidenceReference, &status, &existingCompensation, &policyVersion)
		if errors.Is(err, sql.ErrNoRows) {
			return appfunding.ErrNotFound
		}
		if err != nil {
			return err
		}
		if status != "posted" || existingCompensation != "" {
			return appfunding.ErrConflict
		}
		var mode string
		var financeActive bool
		if err = tx.QueryRowContext(ctx, `SELECT mode,finance_activated FROM tenant_funding_policies WHERE tenant_id=$1 AND currency=$2`, command.TenantID, currency).Scan(&mode, &financeActive); err != nil {
			return err
		}
		if mode == string(appfunding.PolicyProductionDualControl) && !financeActive {
			return appfunding.ErrForbidden
		}
		eventID, err := newUUID()
		if err != nil {
			return err
		}
		compensationReference := "compensation:" + command.FundingEventID
		_, err = tx.ExecContext(ctx, `
INSERT INTO funding_events(
 id,tenant_id,requester_subject_id,destination_account_id,system_account_id,
 external_reference,evidence_reference,idempotency_key,request_fingerprint,
 amount_minor,currency,status,demo_policy,policy_version,compensation_of_event_id,
 compensation_reason_code,compensation_operator_note,correlation_id,requested_at,updated_at
) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'requested',false,$12,$13,$14,$15,$16,$17,$17)`,
			eventID, command.TenantID, command.ActorSubjectID, destinationID, systemID,
			compensationReference, evidenceReference, command.IdempotencyKey, fingerprint[:], amount, currency,
			policyVersion, command.FundingEventID, command.ReasonCode, command.OperatorNote, command.CorrelationID, command.OccurredAt)
		if err != nil {
			return classifyFundingInsertError("insert funding compensation request", err)
		}
		approvalID, err := newUUID()
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `
INSERT INTO approval_records(id,tenant_id,command_type,target_id,requester_subject_id,status,expires_at,correlation_id,policy_version,created_at)
VALUES($1,$2,'funding_compensation',$3,$4,'requested',$5,$6,$7,$8)`, approvalID, command.TenantID, eventID, command.ActorSubjectID, command.OccurredAt.Add(24*time.Hour), command.CorrelationID, policyVersion, command.OccurredAt); err != nil {
			return err
		}
		if err = insertFundingAudit(ctx, tx, command.TenantID, command.ActorSubjectID, eventID, "funding.compensation.requested", "succeeded", command.CorrelationID, command.OccurredAt); err != nil {
			return err
		}
		event, readErr := readFundingEventByID(ctx, tx, command.TenantID, eventID)
		if readErr != nil {
			return readErr
		}
		submission = appfunding.Submission{Event: event}
		return nil
	})
	return submission, err
}

func (r *FundingRepository) Get(ctx context.Context, tenantID, actorID, eventID string) (appfunding.Event, error) {
	if err := authorizeFinanceDatabase(ctx, r.database, tenantID, actorID); err != nil {
		return appfunding.Event{}, err
	}
	event, err := readFundingEventByID(ctx, r.database, tenantID, eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return appfunding.Event{}, appfunding.ErrNotFound
	}
	return event, err
}

func (r *FundingRepository) List(ctx context.Context, tenantID, actorID string, query appfunding.Query) (appfunding.Page, error) {
	if err := authorizeFinanceDatabase(ctx, r.database, tenantID, actorID); err != nil {
		return appfunding.Page{}, err
	}
	cursor, err := decodeFundingCursor(query.Cursor)
	if err != nil {
		return appfunding.Page{}, appfunding.ErrInvalidCommand
	}
	rows, err := r.database.QueryContext(ctx, `SELECT `+fundingEventColumns+` FROM funding_events WHERE tenant_id=$1 AND ($2='' OR status=$2) AND ($3::timestamptz IS NULL OR (requested_at,id)<($3::timestamptz,$4::uuid)) ORDER BY requested_at DESC,id DESC LIMIT $5`, tenantID, query.Status, nullableTime(cursor.RequestedAt), nullableString(cursor.ID), query.Limit+1)
	if err != nil {
		return appfunding.Page{}, err
	}
	defer func() { _ = rows.Close() }()
	page := appfunding.Page{Events: make([]appfunding.Event, 0)}
	for rows.Next() {
		event, err := scanFundingEvent(rows)
		if err != nil {
			return appfunding.Page{}, err
		}
		page.Events = append(page.Events, event)
	}
	if err := rows.Err(); err != nil {
		return appfunding.Page{}, err
	}
	if len(page.Events) > query.Limit {
		last := page.Events[query.Limit-1]
		requestedAt, err := time.Parse(time.RFC3339Nano, last.RequestedAt)
		if err != nil {
			return appfunding.Page{}, err
		}
		page.NextCursor = encodeFundingCursor(requestedAt, last.FundingEventID)
		page.Events = page.Events[:query.Limit]
	}
	return page, nil
}

func (r *FundingRepository) Reconcile(ctx context.Context, tenantID, actorID, eventID string) (appfunding.Reconciliation, error) {
	if err := authorizeFinanceDatabase(ctx, r.database, tenantID, actorID); err != nil {
		return appfunding.Reconciliation{}, err
	}
	var external, currency string
	var expected, debit, credit int64
	err := r.database.QueryRowContext(ctx, `
SELECT event.external_reference,event.currency,event.amount_minor,
 COALESCE(sum(posting.amount_minor) FILTER (WHERE posting.direction='debit'),0),
 COALESCE(sum(posting.amount_minor) FILTER (WHERE posting.direction='credit'),0)
FROM funding_events event
LEFT JOIN journal_transactions journal ON journal.id=event.journal_transaction_id
LEFT JOIN ledger_postings posting ON posting.journal_transaction_id=journal.id
WHERE event.tenant_id=$1 AND event.id=$2 AND event.status IN ('posted','compensated')
GROUP BY event.external_reference,event.currency,event.amount_minor`, tenantID, eventID).Scan(&external, &currency, &expected, &debit, &credit)
	if errors.Is(err, sql.ErrNoRows) {
		return appfunding.Reconciliation{}, appfunding.ErrNotFound
	}
	if err != nil {
		return appfunding.Reconciliation{}, err
	}
	status := "mismatch"
	if expected == debit && expected == credit {
		status = "matched"
	}
	return appfunding.Reconciliation{FundingEventID: eventID, ExternalReference: external, Status: status, ExpectedMinor: strconv.FormatInt(expected, 10), PostedDebitMinor: strconv.FormatInt(debit, 10), PostedCreditMinor: strconv.FormatInt(credit, 10), Currency: currency, CheckedAt: r.clock().UTC().Format(time.RFC3339Nano)}, nil
}

func authorizeFinanceActor(ctx context.Context, tx *sql.Tx, tenantID, actorID string) error {
	var authorized bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tenant_subject_roles WHERE tenant_id=$1 AND subject_id=$2 AND role='finance')`, tenantID, actorID).Scan(&authorized); err != nil {
		return err
	}
	if !authorized {
		return appfunding.ErrForbidden
	}
	return nil
}

func authorizeFinanceDatabase(ctx context.Context, database *sql.DB, tenantID, actorID string) error {
	var authorized bool
	if err := database.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tenant_subject_roles WHERE tenant_id=$1 AND subject_id=$2 AND role='finance')`, tenantID, actorID).Scan(&authorized); err != nil {
		return err
	}
	if !authorized {
		return appfunding.ErrNotFound
	}
	return nil
}

func loadFundingPolicy(ctx context.Context, tx *sql.Tx, tenantID, destinationID, currency string) (fundingPolicy, error) {
	var policy fundingPolicy
	err := tx.QueryRowContext(ctx, `
SELECT policy.mode,policy.finance_activated,policy.policy_version,policy.per_command_minor,policy.operator_rolling_24h_minor,policy.tenant_rolling_24h_minor
FROM accounts account JOIN tenant_funding_policies policy ON policy.tenant_id=account.tenant_id AND policy.currency=account.currency
WHERE account.tenant_id=$1 AND account.id=$2 AND account.currency=$3 AND account.status='active' AND account.account_kind='customer'
FOR UPDATE OF account`, tenantID, destinationID, currency).Scan(&policy.Mode, &policy.FinanceActive, &policy.Version, &policy.PerCommand, &policy.OperatorRolling, &policy.TenantRolling)
	if errors.Is(err, sql.ErrNoRows) {
		return policy, appfunding.ErrNotFound
	}
	return policy, err
}

func ensureFundingAccount(ctx context.Context, tx *sql.Tx, tenantID, currency string, now time.Time) (string, error) {
	var accountID string
	err := tx.QueryRowContext(ctx, `SELECT id::text FROM accounts WHERE tenant_id=$1 AND currency=$2 AND account_kind='funding_clearing' FOR UPDATE`, tenantID, currency).Scan(&accountID)
	if err == nil {
		return accountID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	accountID, err = newUUID()
	if err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO accounts(id,tenant_id,currency,status,display_name,category,external_reference,account_kind,created_at,updated_at) VALUES($1,$2,$3,'active',$4,'system',$5,'funding_clearing',$6,$6)`, accountID, tenantID, currency, "Funding clearing · "+currency, "system-funding-"+strings.ToLower(currency), now); err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO account_balance_projections(account_id,available_minor,ledger_minor,balance_version,allow_negative,updated_at) VALUES($1,0,0,0,true,$2)`, accountID, now); err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO account_opening_balances(account_id,opening_ledger_minor) VALUES($1,0)`, accountID); err != nil {
		return "", err
	}
	return accountID, nil
}

func readFundingEvent(ctx context.Context, query interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, tenantID, idempotencyKey, actorID string) (appfunding.Event, error) {
	return scanFundingEvent(query.QueryRowContext(ctx, `SELECT `+fundingEventColumns+` FROM funding_events WHERE tenant_id=$1 AND idempotency_key=$2 AND requester_subject_id=$3`, tenantID, idempotencyKey, actorID))
}

func readFundingEventByID(ctx context.Context, query interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, tenantID, eventID string) (appfunding.Event, error) {
	return scanFundingEvent(query.QueryRowContext(ctx, `SELECT `+fundingEventColumns+` FROM funding_events WHERE tenant_id=$1 AND id=$2`, tenantID, eventID))
}

func scanFundingEvent(scanner fundingRowScanner) (appfunding.Event, error) {
	var event appfunding.Event
	var amount int64
	var requestedAt, updatedAt time.Time
	err := scanner.Scan(&event.FundingEventID, &event.Status, &event.DestinationAccountID, &event.SystemAccountID, &event.Currency, &amount, &event.ExternalReference, &event.EvidenceReference, &event.RequesterSubjectID, &event.ApproverSubjectID, &event.DecisionReason, &event.DemoPolicy, &event.JournalTransactionID, &event.CompensationOfEventID, &event.CompensationEventID, &event.CompensationReasonCode, &event.CompensationOperatorNote, &requestedAt, &updatedAt, &event.BalanceVersion)
	if err != nil {
		return event, err
	}
	event.AmountMinor = strconv.FormatInt(amount, 10)
	event.RequestedAt, event.UpdatedAt = requestedAt.UTC().Format(time.RFC3339Nano), updatedAt.UTC().Format(time.RFC3339Nano)
	return event, nil
}

func fundingCommandType(compensationOf string) string {
	if compensationOf != "" {
		return "funding_compensation"
	}
	return "funding"
}

func classifyFundingInsertError(operation string, err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return appfunding.ErrConflict
	}
	return fmt.Errorf("%s: %w", operation, err)
}

type fundingCursor struct {
	RequestedAt time.Time `json:"requested_at"`
	ID          string    `json:"id"`
}

func encodeFundingCursor(requestedAt time.Time, id string) string {
	payload, _ := json.Marshal(fundingCursor{RequestedAt: requestedAt.UTC(), ID: id})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeFundingCursor(value string) (fundingCursor, error) {
	if value == "" {
		return fundingCursor{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return fundingCursor{}, err
	}
	var cursor fundingCursor
	if err = json.Unmarshal(payload, &cursor); err != nil || cursor.RequestedAt.IsZero() || cursor.ID == "" {
		return fundingCursor{}, errors.New("invalid funding cursor")
	}
	return cursor, nil
}

func insertFundingAudit(ctx context.Context, tx *sql.Tx, tenantID, actorID, eventID, eventType, outcome, correlationID string, occurredAt time.Time) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	metadata, _ := json.Marshal(map[string]string{"funding_event_id": eventID, "terminology": "recorded external value evidence"})
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at) VALUES($1,$2,$3,$4,'funding_event',$5,$6,$7,$8,$9)`, id, tenantID, actorID, eventType, eventID, outcome, correlationID, metadata, occurredAt)
	return err
}

func enqueueFundingBalanceEvent(ctx context.Context, tx *sql.Tx, command appfunding.ActionCommand, currency string, amount int64, balance lockedAccount, occurredAt time.Time) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{
		"event_id": id, "event_type": "account.balance.changed.v1", "account_id": balance.ID,
		"funding_event_id": command.FundingEventID, "currency": currency,
		"available_minor": strconv.FormatInt(balance.AvailableMinor, 10), "balance_version": strconv.FormatInt(balance.BalanceVersion, 10),
		"occurred_at": occurredAt.Format(time.RFC3339Nano), "amount_minor": strconv.FormatInt(amount, 10),
	})
	_, err = tx.ExecContext(ctx, `INSERT INTO outbox_events(id,tenant_id,funding_event_id,account_id,aggregate_type,aggregate_id,event_type,aggregate_version,payload,occurred_at) VALUES($1,$2,$3,$4,'account_balance',$4,'account.balance.changed.v1',$5,$6,$7)`, id, command.TenantID, command.FundingEventID, balance.ID, balance.BalanceVersion, payload, occurredAt)
	return err
}
