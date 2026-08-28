package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	appcorrections "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/corrections"
	apptransfers "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/ledger"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

const correctionColumns = `
c.id::text,c.original_transfer_id::text,COALESCE(c.compensation_transfer_id::text,''),
original.journal_transaction_id::text,COALESCE(compensation.journal_transaction_id::text,''),
c.requester_subject_id,COALESCE(c.approver_subject_id,''),original.debit_account_id::text,
original.credit_account_id::text,original.amount_minor::text,original.currency,c.reason_code,c.operator_note,
COALESCE(c.decision_reason,''),c.status,c.policy_version::text,c.control_mode,c.step_up_required,
c.approval_expires_at,c.requested_at,c.updated_at`

type TransferCorrectionRepository struct {
	database *sql.DB
	clock    func() time.Time
}

func NewTransferCorrectionRepository(database *sql.DB, clock func() time.Time) (*TransferCorrectionRepository, error) {
	if database == nil {
		return nil, errors.New("transfer correction database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &TransferCorrectionRepository{database: database, clock: clock}, nil
}

func (r *TransferCorrectionRepository) Request(ctx context.Context, command appcorrections.RequestCommand, fingerprint [sha256.Size]byte) (submission appcorrections.Submission, err error) {
	err = WithSerializableSequence(ctx, r.database, "transfer-correction|"+strings.ToLower(command.TenantID)+"|"+command.OriginalTransferID, 5, func(tx *sql.Tx) error {
		if err := authorizeCorrectionRequester(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		var existing []byte
		existingErr := tx.QueryRowContext(ctx, `SELECT request_fingerprint FROM transfer_corrections WHERE tenant_id=$1 AND requester_subject_id=$2 AND idempotency_key=$3`, command.TenantID, command.ActorSubjectID, command.IdempotencyKey).Scan(&existing)
		if existingErr == nil {
			if string(existing) != string(fingerprint[:]) {
				return appcorrections.ErrConflict
			}
			event, err := readCorrectionByIdempotency(ctx, tx, command.TenantID, command.ActorSubjectID, command.IdempotencyKey)
			if err == nil {
				submission = appcorrections.Submission{Event: event, Replayed: true}
			}
			return err
		}
		if !errors.Is(existingErr, sql.ErrNoRows) {
			return existingErr
		}
		var status, compensationOf, mode string
		var policyVersion int64
		var stepUpRequired bool
		var ttlMinutes int
		err := tx.QueryRowContext(ctx, `
SELECT original.status,COALESCE(original.compensation_of_transfer_id::text,''),policy.policy_version,
 policy.control_mode,policy.requires_step_up,policy.approval_ttl_minutes
FROM transfers original JOIN tenant_transfer_policies policy ON policy.tenant_id=original.tenant_id
WHERE original.tenant_id=$1 AND original.id=$2 FOR UPDATE OF original`, command.TenantID, command.OriginalTransferID).
			Scan(&status, &compensationOf, &policyVersion, &mode, &stepUpRequired, &ttlMinutes)
		if errors.Is(err, sql.ErrNoRows) {
			return appcorrections.ErrNotFound
		}
		if err != nil {
			return err
		}
		if status != "posted" || compensationOf != "" {
			return appcorrections.ErrConflict
		}
		if mode == "production_dual_control" && stepUpRequired && !recentStepUp(command.StepUpAuthenticatedAt, command.RequestedAt) {
			return appcorrections.ErrStepUpRequired
		}
		id, err := newUUID()
		if err != nil {
			return err
		}
		expires := command.RequestedAt.Add(time.Duration(ttlMinutes) * time.Minute)
		_, err = tx.ExecContext(ctx, `
INSERT INTO transfer_corrections(id,tenant_id,original_transfer_id,requester_subject_id,reason_code,operator_note,
 idempotency_key,request_fingerprint,status,policy_version,control_mode,step_up_required,approval_expires_at,
 correlation_id,requested_at,updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,'requested',$9,$10,$11,$12,$13,$14,$14)`,
			id, command.TenantID, command.OriginalTransferID, command.ActorSubjectID, command.ReasonCode, command.OperatorNote,
			command.IdempotencyKey, fingerprint[:], policyVersion, mode, stepUpRequired, expires, command.CorrelationID, command.RequestedAt)
		if err != nil {
			return classifyCorrectionInsert(err)
		}
		if err = insertCorrectionApproval(ctx, tx, command.TenantID, id, command.ActorSubjectID, "", "requested", expires, "", command.CorrelationID, policyVersion, command.RequestedAt); err != nil {
			return err
		}
		if err = insertCorrectionAudit(ctx, tx, command.TenantID, command.ActorSubjectID, id, "transfer_correction.requested", "succeeded", command.CorrelationID, command.RequestedAt, map[string]any{"reason_code": command.ReasonCode, "step_up_verified": !stepUpRequired || recentStepUp(command.StepUpAuthenticatedAt, command.RequestedAt)}); err != nil {
			return err
		}
		event, err := readCorrectionByID(ctx, tx, command.TenantID, id)
		if err == nil {
			submission = appcorrections.Submission{Event: event}
		}
		return err
	})
	return submission, err
}

func (r *TransferCorrectionRepository) Approve(ctx context.Context, command appcorrections.DecisionCommand) (event appcorrections.Event, err error) {
	err = WithSerializableSequence(ctx, r.database, "transfer-correction-decision|"+strings.ToLower(command.TenantID)+"|"+command.CorrectionID, 5, func(tx *sql.Tx) error {
		if err := authorizeCorrectionApprover(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		var requester, status, mode string
		var expires time.Time
		var stepUp bool
		var version int64
		err := tx.QueryRowContext(ctx, `SELECT requester_subject_id,status,control_mode,step_up_required,approval_expires_at,policy_version FROM transfer_corrections WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, command.TenantID, command.CorrectionID).Scan(&requester, &status, &mode, &stepUp, &expires, &version)
		if errors.Is(err, sql.ErrNoRows) {
			return appcorrections.ErrNotFound
		}
		if err != nil {
			return err
		}
		if status == "approved" {
			event, err = readCorrectionByID(ctx, tx, command.TenantID, command.CorrectionID)
			return err
		}
		if status != "requested" {
			return appcorrections.ErrConflict
		}
		if !command.DecidedAt.Before(expires) {
			return r.expire(ctx, tx, command.TenantID, command.ActorSubjectID, command.CorrectionID, requester, command.CorrelationID, version, command.DecidedAt, &event)
		}
		if mode == "production_dual_control" && requester == command.ActorSubjectID {
			return appcorrections.ErrForbidden
		}
		if mode == "production_dual_control" && stepUp && !recentStepUp(command.StepUpAuthenticatedAt, command.DecidedAt) {
			return appcorrections.ErrStepUpRequired
		}
		result, err := tx.ExecContext(ctx, `UPDATE transfer_corrections SET status='approved',approver_subject_id=$3,decision_reason=$4,decided_at=$5,updated_at=$5 WHERE tenant_id=$1 AND id=$2 AND status='requested'`, command.TenantID, command.CorrectionID, command.ActorSubjectID, command.Reason, command.DecidedAt)
		if err != nil {
			return err
		}
		if err = requireOneRow(result, "approve correction"); err != nil {
			return appcorrections.ErrConflict
		}
		if err = insertCorrectionApproval(ctx, tx, command.TenantID, command.CorrectionID, requester, command.ActorSubjectID, "approved", expires, command.Reason, command.CorrelationID, version, command.DecidedAt); err != nil {
			return err
		}
		if err = insertCorrectionAudit(ctx, tx, command.TenantID, command.ActorSubjectID, command.CorrectionID, "transfer_correction.approved", "succeeded", command.CorrelationID, command.DecidedAt, map[string]any{"step_up_verified": !stepUp || recentStepUp(command.StepUpAuthenticatedAt, command.DecidedAt)}); err != nil {
			return err
		}
		event, err = readCorrectionByID(ctx, tx, command.TenantID, command.CorrectionID)
		return err
	})
	return event, err
}

func (r *TransferCorrectionRepository) Reject(ctx context.Context, command appcorrections.DecisionCommand) (event appcorrections.Event, err error) {
	err = WithSerializableSequence(ctx, r.database, "transfer-correction-decision|"+strings.ToLower(command.TenantID)+"|"+command.CorrectionID, 5, func(tx *sql.Tx) error {
		if err := authorizeCorrectionApprover(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		var requester, status, mode string
		var expires time.Time
		var stepUp bool
		var version int64
		err := tx.QueryRowContext(ctx, `SELECT requester_subject_id,status,control_mode,step_up_required,approval_expires_at,policy_version FROM transfer_corrections WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, command.TenantID, command.CorrectionID).Scan(&requester, &status, &mode, &stepUp, &expires, &version)
		if errors.Is(err, sql.ErrNoRows) {
			return appcorrections.ErrNotFound
		}
		if err != nil {
			return err
		}
		if status != "requested" {
			return appcorrections.ErrConflict
		}
		if !command.DecidedAt.Before(expires) {
			return r.expire(ctx, tx, command.TenantID, command.ActorSubjectID, command.CorrectionID, requester, command.CorrelationID, version, command.DecidedAt, &event)
		}
		if mode == "production_dual_control" && requester == command.ActorSubjectID {
			return appcorrections.ErrForbidden
		}
		if mode == "production_dual_control" && stepUp && !recentStepUp(command.StepUpAuthenticatedAt, command.DecidedAt) {
			return appcorrections.ErrStepUpRequired
		}
		result, err := tx.ExecContext(ctx, `UPDATE transfer_corrections SET status='rejected',approver_subject_id=$3,decision_reason=$4,decided_at=$5,updated_at=$5 WHERE tenant_id=$1 AND id=$2 AND status='requested'`, command.TenantID, command.CorrectionID, command.ActorSubjectID, command.Reason, command.DecidedAt)
		if err != nil {
			return err
		}
		if err = requireOneRow(result, "reject correction"); err != nil {
			return appcorrections.ErrConflict
		}
		if err = insertCorrectionApproval(ctx, tx, command.TenantID, command.CorrectionID, requester, command.ActorSubjectID, "rejected", expires, command.Reason, command.CorrelationID, version, command.DecidedAt); err != nil {
			return err
		}
		if err = insertCorrectionAudit(ctx, tx, command.TenantID, command.ActorSubjectID, command.CorrectionID, "transfer_correction.rejected", "succeeded", command.CorrelationID, command.DecidedAt, map[string]any{"step_up_verified": !stepUp || recentStepUp(command.StepUpAuthenticatedAt, command.DecidedAt)}); err != nil {
			return err
		}
		event, err = readCorrectionByID(ctx, tx, command.TenantID, command.CorrectionID)
		return err
	})
	return event, err
}

func (r *TransferCorrectionRepository) Cancel(ctx context.Context, command appcorrections.CancelCommand) (event appcorrections.Event, err error) {
	err = WithSerializableSequence(ctx, r.database, "transfer-correction-decision|"+strings.ToLower(command.TenantID)+"|"+command.CorrectionID, 5, func(tx *sql.Tx) error {
		var requester, status string
		var expires time.Time
		var version int64
		err := tx.QueryRowContext(ctx, `SELECT requester_subject_id,status,approval_expires_at,policy_version FROM transfer_corrections WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, command.TenantID, command.CorrectionID).Scan(&requester, &status, &expires, &version)
		if errors.Is(err, sql.ErrNoRows) {
			return appcorrections.ErrNotFound
		}
		if err != nil {
			return err
		}
		if requester != command.ActorSubjectID {
			return appcorrections.ErrForbidden
		}
		if status != "requested" && status != "approved" {
			return appcorrections.ErrConflict
		}
		result, err := tx.ExecContext(ctx, `UPDATE transfer_corrections SET status='cancelled',decision_reason=$3,cancelled_at=$4,updated_at=$4 WHERE tenant_id=$1 AND id=$2 AND status IN ('requested','approved')`, command.TenantID, command.CorrectionID, command.Reason, command.CancelledAt)
		if err != nil {
			return err
		}
		if err = requireOneRow(result, "cancel correction"); err != nil {
			return appcorrections.ErrConflict
		}
		if err = insertCorrectionApproval(ctx, tx, command.TenantID, command.CorrectionID, requester, "", "cancelled", expires, command.Reason, command.CorrelationID, version, command.CancelledAt); err != nil {
			return err
		}
		if err = insertCorrectionAudit(ctx, tx, command.TenantID, command.ActorSubjectID, command.CorrectionID, "transfer_correction.cancelled", "succeeded", command.CorrelationID, command.CancelledAt, nil); err != nil {
			return err
		}
		event, err = readCorrectionByID(ctx, tx, command.TenantID, command.CorrectionID)
		return err
	})
	return event, err
}

func (r *TransferCorrectionRepository) Post(ctx context.Context, command appcorrections.PostCommand) (submission appcorrections.Submission, err error) {
	err = WithSerializableSequence(ctx, r.database, "transfer-correction-post|"+strings.ToLower(command.TenantID)+"|"+command.CorrectionID, 5, func(tx *sql.Tx) error {
		if err := authorizeCorrectionApprover(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		var requester, approver, status, mode, originalID, debitID, creditID, currency string
		var amount, version int64
		var stepUp bool
		var expires time.Time
		err := tx.QueryRowContext(ctx, `
SELECT correction.requester_subject_id,COALESCE(correction.approver_subject_id,''),correction.status,correction.control_mode,
 correction.step_up_required,correction.approval_expires_at,correction.policy_version,original.id::text,
 original.debit_account_id::text,original.credit_account_id::text,original.amount_minor,original.currency
FROM transfer_corrections correction JOIN transfers original ON original.id=correction.original_transfer_id
WHERE correction.tenant_id=$1 AND correction.id=$2 FOR UPDATE OF correction,original`, command.TenantID, command.CorrectionID).
			Scan(&requester, &approver, &status, &mode, &stepUp, &expires, &version, &originalID, &debitID, &creditID, &amount, &currency)
		if errors.Is(err, sql.ErrNoRows) {
			return appcorrections.ErrNotFound
		}
		if err != nil {
			return err
		}
		if status == "posted" {
			event, err := readCorrectionByID(ctx, tx, command.TenantID, command.CorrectionID)
			if err == nil {
				submission = appcorrections.Submission{Event: event, Replayed: true}
			}
			return err
		}
		if status != "approved" || approver == "" {
			return appcorrections.ErrConflict
		}
		if mode == "production_dual_control" && requester == command.ActorSubjectID {
			return appcorrections.ErrForbidden
		}
		if !command.OccurredAt.Before(expires) {
			var event appcorrections.Event
			err = r.expire(ctx, tx, command.TenantID, command.ActorSubjectID, command.CorrectionID, requester, command.CorrelationID, version, command.OccurredAt, &event)
			submission = appcorrections.Submission{Event: event}
			return err
		}
		if mode == "production_dual_control" && stepUp && !recentStepUp(command.StepUpAuthenticatedAt, command.OccurredAt) {
			return appcorrections.ErrStepUpRequired
		}
		accounts, err := lockAccounts(ctx, tx, command.TenantID, creditID, debitID)
		if err != nil {
			return err
		}
		source, destination := accounts[creditID], accounts[debitID]
		if source.Status == "closed" || destination.Status == "closed" || source.Currency != currency || destination.Currency != currency {
			return appcorrections.ErrConflict
		}
		amountValue, err := money.New(currency, amount)
		if err != nil {
			return err
		}
		compensationID, err := newUUID()
		if err != nil {
			return err
		}
		journalID, err := newUUID()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO transfers(id,tenant_id,actor_subject_id,debit_account_id,credit_account_id,amount_minor,currency,status,
 journal_transaction_id,created_at,completed_at,policy_version,compensation_of_transfer_id)
VALUES($1,$2,$3,$4,$5,$6,$7,'posted',$8,$9,$9,$10,$11)`, compensationID, command.TenantID, command.ActorSubjectID, creditID, debitID, amount, currency, journalID, command.OccurredAt, version, originalID)
		if err != nil {
			return classifyCorrectionInsert(err)
		}
		if err = createJournal(ctx, tx, journalID, command.TenantID, compensationID, command.OccurredAt); err != nil {
			return err
		}
		debitPostingID, err := newUUID()
		if err != nil {
			return err
		}
		creditPostingID, err := newUUID()
		if err != nil {
			return err
		}
		debitPosting, err := ledger.NewPosting(debitPostingID, journalID, creditID, ledger.Debit, amountValue, command.OccurredAt)
		if err != nil {
			return err
		}
		creditPosting, err := ledger.NewPosting(creditPostingID, journalID, debitID, ledger.Credit, amountValue, command.OccurredAt)
		if err != nil {
			return err
		}
		if err = ledger.ValidateBalanced([]ledger.Posting{debitPosting, creditPosting}); err != nil {
			return err
		}
		if err = createPosting(ctx, tx, debitPosting); err != nil {
			return err
		}
		if err = createPosting(ctx, tx, creditPosting); err != nil {
			return err
		}
		updatedSource, err := applyBalanceDelta(ctx, tx, creditID, -amount, command.OccurredAt)
		if err != nil {
			return appcorrections.ErrConflict
		}
		updatedDestination, err := applyBalanceDelta(ctx, tx, debitID, amount, command.OccurredAt)
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `UPDATE transfer_corrections SET status='posted',compensation_transfer_id=$3,posted_at=$4,updated_at=$4 WHERE tenant_id=$1 AND id=$2 AND status='approved'`, command.TenantID, command.CorrectionID, compensationID, command.OccurredAt)
		if err != nil {
			return err
		}
		if err = requireOneRow(result, "post correction"); err != nil {
			return appcorrections.ErrConflict
		}
		transferCommand := apptransfers.Command{TenantID: command.TenantID, ActorSubjectID: command.ActorSubjectID, DebitAccountID: creditID, CreditAccountID: debitID, Amount: amountValue, CorrelationID: command.CorrelationID, OccurredAt: command.OccurredAt}
		if err = enqueueBalanceEvent(ctx, tx, transferCommand, compensationID, updatedSource, command.OccurredAt); err != nil {
			return err
		}
		if err = enqueueBalanceEvent(ctx, tx, transferCommand, compensationID, updatedDestination, command.OccurredAt); err != nil {
			return err
		}
		if err = insertCorrectionAudit(ctx, tx, command.TenantID, command.ActorSubjectID, command.CorrectionID, "transfer_correction.posted", "succeeded", command.CorrelationID, command.OccurredAt, map[string]any{"original_transfer_id": originalID, "compensation_transfer_id": compensationID, "step_up_verified": !stepUp || recentStepUp(command.StepUpAuthenticatedAt, command.OccurredAt)}); err != nil {
			return err
		}
		event, err := readCorrectionByID(ctx, tx, command.TenantID, command.CorrectionID)
		if err == nil {
			submission = appcorrections.Submission{Event: event}
		}
		return err
	})
	return submission, err
}

func (r *TransferCorrectionRepository) Get(ctx context.Context, tenantID, actorID, correctionID string) (appcorrections.Event, error) {
	if err := authorizeCorrectionReaderDB(ctx, r.database, tenantID, actorID); err != nil {
		return appcorrections.Event{}, err
	}
	return readCorrectionByID(ctx, r.database, tenantID, correctionID)
}
func (r *TransferCorrectionRepository) List(ctx context.Context, tenantID, actorID string, query appcorrections.Query) (appcorrections.Page, error) {
	if err := authorizeCorrectionReaderDB(ctx, r.database, tenantID, actorID); err != nil {
		return appcorrections.Page{}, err
	}
	var before time.Time
	var beforeID string
	if query.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(query.Cursor)
		if err != nil {
			return appcorrections.Page{}, appcorrections.ErrInvalidCommand
		}
		parts := strings.Split(string(raw), "|")
		if len(parts) != 2 {
			return appcorrections.Page{}, appcorrections.ErrInvalidCommand
		}
		before, err = time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return appcorrections.Page{}, appcorrections.ErrInvalidCommand
		}
		beforeID = parts[1]
	}
	rows, err := r.database.QueryContext(ctx, `SELECT `+correctionColumns+` FROM transfer_corrections c JOIN transfers original ON original.id=c.original_transfer_id LEFT JOIN transfers compensation ON compensation.id=c.compensation_transfer_id WHERE c.tenant_id=$1 AND ($2='' OR c.status=$2) AND ($3::timestamptz IS NULL OR (c.requested_at,c.id)<($3::timestamptz,$4::uuid)) ORDER BY c.requested_at DESC,c.id DESC LIMIT $5`, tenantID, query.Status, nullableTime(before), nullableString(beforeID), query.Limit+1)
	if err != nil {
		return appcorrections.Page{}, err
	}
	defer func() { _ = rows.Close() }()
	page := appcorrections.Page{}
	for rows.Next() {
		event, err := scanCorrection(rows)
		if err != nil {
			return appcorrections.Page{}, err
		}
		page.Events = append(page.Events, event)
	}
	if err = rows.Err(); err != nil {
		return appcorrections.Page{}, err
	}
	if len(page.Events) > query.Limit {
		last := page.Events[query.Limit-1]
		page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(last.RequestedAt + "|" + last.CorrectionID))
		page.Events = page.Events[:query.Limit]
	}
	return page, nil
}

type correctionQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
type correctionScanner interface{ Scan(...any) error }

func readCorrectionByID(ctx context.Context, q correctionQueryer, tenantID, id string) (appcorrections.Event, error) {
	row := q.QueryRowContext(ctx, `SELECT `+correctionColumns+` FROM transfer_corrections c JOIN transfers original ON original.id=c.original_transfer_id LEFT JOIN transfers compensation ON compensation.id=c.compensation_transfer_id WHERE c.tenant_id=$1 AND c.id=$2`, tenantID, id)
	event, err := scanCorrection(row)
	if errors.Is(err, sql.ErrNoRows) {
		return appcorrections.Event{}, appcorrections.ErrNotFound
	}
	return event, err
}
func readCorrectionByIdempotency(ctx context.Context, q correctionQueryer, tenantID, actorID, key string) (appcorrections.Event, error) {
	row := q.QueryRowContext(ctx, `SELECT `+correctionColumns+` FROM transfer_corrections c JOIN transfers original ON original.id=c.original_transfer_id LEFT JOIN transfers compensation ON compensation.id=c.compensation_transfer_id WHERE c.tenant_id=$1 AND c.requester_subject_id=$2 AND c.idempotency_key=$3`, tenantID, actorID, key)
	return scanCorrection(row)
}
func scanCorrection(scanner correctionScanner) (appcorrections.Event, error) {
	var event appcorrections.Event
	var expires, requested, updated time.Time
	err := scanner.Scan(&event.CorrectionID, &event.OriginalTransferID, &event.CompensationTransferID, &event.OriginalJournalID, &event.CompensationJournalID, &event.RequesterSubjectID, &event.ApproverSubjectID, &event.DebitAccountID, &event.CreditAccountID, &event.AmountMinor, &event.Currency, &event.ReasonCode, &event.OperatorNote, &event.DecisionReason, &event.Status, &event.PolicyVersion, &event.ControlMode, &event.StepUpRequired, &expires, &requested, &updated)
	if err != nil {
		return appcorrections.Event{}, err
	}
	event.ApprovalExpiresAt = expires.UTC().Format(time.RFC3339Nano)
	event.RequestedAt = requested.UTC().Format(time.RFC3339Nano)
	event.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
	return event, nil
}

func (r *TransferCorrectionRepository) expire(ctx context.Context, tx *sql.Tx, tenantID, actorID, correctionID, requester, correlationID string, version int64, at time.Time, event *appcorrections.Event) error {
	result, err := tx.ExecContext(ctx, `UPDATE transfer_corrections SET status='expired',decision_reason='approval_window_expired',cancelled_at=$3,updated_at=$3 WHERE tenant_id=$1 AND id=$2 AND status IN ('requested','approved')`, tenantID, correctionID, at)
	if err != nil {
		return err
	}
	if err = requireOneRow(result, "expire correction"); err != nil {
		return appcorrections.ErrConflict
	}
	if err = insertCorrectionApproval(ctx, tx, tenantID, correctionID, requester, "", "expired", at, "approval_window_expired", correlationID, version, at); err != nil {
		return err
	}
	if err = insertCorrectionAudit(ctx, tx, tenantID, actorID, correctionID, "transfer_correction.expired", "failed", correlationID, at, nil); err != nil {
		return err
	}
	*event, err = readCorrectionByID(ctx, tx, tenantID, correctionID)
	return err
}
func recentStepUp(authenticatedAt, now time.Time) bool {
	return !authenticatedAt.IsZero() && !authenticatedAt.After(now.Add(time.Minute)) && now.Sub(authenticatedAt) <= 10*time.Minute
}
func authorizeCorrectionRequester(ctx context.Context, tx *sql.Tx, tenantID, actorID string) error {
	return authorizeCorrectionRole(ctx, tx, tenantID, actorID, false)
}
func authorizeCorrectionApprover(ctx context.Context, tx *sql.Tx, tenantID, actorID string) error {
	return authorizeCorrectionRole(ctx, tx, tenantID, actorID, true)
}
func authorizeCorrectionRole(ctx context.Context, tx *sql.Tx, tenantID, actorID string, financeOnly bool) error {
	var allowed bool
	query := `SELECT EXISTS(SELECT 1 FROM tenant_subject_roles WHERE tenant_id=$1 AND subject_id=$2 AND role IN ('operator','finance'))`
	if financeOnly {
		query = `SELECT EXISTS(SELECT 1 FROM tenant_subject_roles WHERE tenant_id=$1 AND subject_id=$2 AND role='finance')`
	}
	err := tx.QueryRowContext(ctx, query, tenantID, actorID).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return appcorrections.ErrForbidden
	}
	return nil
}
func authorizeCorrectionReaderDB(ctx context.Context, database *sql.DB, tenantID, actorID string) error {
	var allowed bool
	err := database.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tenant_subject_roles WHERE tenant_id=$1 AND subject_id=$2 AND role IN ('operator','finance','auditor'))`, tenantID, actorID).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return appcorrections.ErrNotFound
	}
	return nil
}
func insertCorrectionApproval(ctx context.Context, tx *sql.Tx, tenantID, targetID, requester, approver, status string, expires time.Time, reason, correlationID string, version int64, at time.Time) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	var decidedAt any
	if status != "requested" {
		decidedAt = at
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO approval_records(id,tenant_id,command_type,target_id,requester_subject_id,approver_subject_id,status,expires_at,decision_reason,correlation_id,policy_version,created_at,decided_at) VALUES($1,$2,'transfer_compensation',$3,$4,NULLIF($5,''),$6,$7,NULLIF($8,''),$9,$10,$11,$12)`, id, tenantID, targetID, requester, approver, status, expires, reason, correlationID, version, at, decidedAt)
	return err
}
func insertCorrectionAudit(ctx context.Context, tx *sql.Tx, tenantID, actorID, targetID, eventType, outcome, correlationID string, at time.Time, metadata map[string]any) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at) VALUES($1,$2,$3,$4,'transfer_correction',$5,$6,$7,$8,$9)`, id, tenantID, actorID, eventType, targetID, outcome, correlationID, encoded, at)
	return err
}
func classifyCorrectionInsert(err error) error {
	if err == nil {
		return nil
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return appcorrections.ErrConflict
	}
	return fmt.Errorf("persist transfer correction: %w", err)
}
