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
	err = WithTenantSerializableSequence(ctx, r.database, command.TenantID, sequence, 5, func(tx *sql.Tx) error {
		var eventID string
		var replayed bool
		if err := tx.QueryRowContext(ctx, `SELECT funding_event_id::text,replayed FROM public.controlled_request_funding_v1($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			command.TenantID, command.ActorSubjectID, command.DestinationAccountID, command.Amount.Minor(), command.Amount.Currency().Code,
			command.ExternalReference, command.EvidenceReference, command.IdempotencyKey, fingerprint[:], command.CorrelationID, command.RequestedAt,
		).Scan(&eventID, &replayed); err != nil {
			return classifyControlledFundingLifecycleError(err)
		}
		event, err := readFundingEventByID(ctx, tx, command.TenantID, eventID)
		if err != nil {
			return err
		}
		submission = appfunding.Submission{Event: event, Replayed: replayed}
		return nil
	})
	return submission, err
}

func (r *FundingRepository) Approve(ctx context.Context, command appfunding.DecisionCommand, demo bool) (event appfunding.Event, err error) {
	err = WithTenantSerializableSequence(ctx, r.database, command.TenantID, "funding-decision|"+strings.ToLower(command.TenantID)+"|"+command.FundingEventID, 5, func(tx *sql.Tx) error {
		_ = demo
		if _, err := tx.ExecContext(ctx, `SELECT public.controlled_decide_funding_v1($1,$2,$3,'approve',$4,$5,$6)`, command.TenantID, command.ActorSubjectID, command.FundingEventID, command.Reason, command.CorrelationID, command.DecidedAt); err != nil {
			return classifyControlledFundingLifecycleError(err)
		}
		var err error
		event, err = readFundingEventByID(ctx, tx, command.TenantID, command.FundingEventID)
		return err
	})
	return event, err
}

func (r *FundingRepository) Reject(ctx context.Context, command appfunding.DecisionCommand) (event appfunding.Event, err error) {
	err = WithTenantSerializableSequence(ctx, r.database, command.TenantID, "funding-decision|"+strings.ToLower(command.TenantID)+"|"+command.FundingEventID, 5, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `SELECT public.controlled_decide_funding_v1($1,$2,$3,'reject',$4,$5,$6)`, command.TenantID, command.ActorSubjectID, command.FundingEventID, command.Reason, command.CorrelationID, command.DecidedAt); err != nil {
			return classifyControlledFundingLifecycleError(err)
		}
		var err error
		event, err = readFundingEventByID(ctx, tx, command.TenantID, command.FundingEventID)
		return err
	})
	return event, err
}

func (r *FundingRepository) Post(ctx context.Context, command appfunding.ActionCommand) (submission appfunding.Submission, err error) {
	err = WithTenantSerializableSequence(ctx, r.database, command.TenantID, "funding-post|"+strings.ToLower(command.TenantID), 5, func(tx *sql.Tx) error {
		var replayed bool
		if queryErr := tx.QueryRowContext(ctx, `SELECT replayed FROM public.controlled_post_funding_v1($1,$2,$3,$4,$5,$6)`,
			command.TenantID, command.ActorSubjectID, command.FundingEventID,
			command.IdempotencyKey, command.CorrelationID, command.OccurredAt,
		).Scan(&replayed); queryErr != nil {
			return classifyControlledFundingError(queryErr)
		}
		event, readErr := readFundingEventByID(ctx, tx, command.TenantID, command.FundingEventID)
		if readErr != nil {
			return readErr
		}
		submission = appfunding.Submission{Event: event, Replayed: replayed}
		return nil
	})
	return submission, err
}

func classifyControlledFundingError(err error) error {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return fmt.Errorf("execute controlled funding post: %w", err)
	}
	switch postgresError.ConstraintName {
	case "controlled_funding_actor", "controlled_funding_forbidden":
		return appfunding.ErrForbidden
	case "controlled_funding_not_found":
		return appfunding.ErrNotFound
	case "controlled_funding_idempotency", "controlled_funding_balance":
		return appfunding.ErrConflict
	case "controlled_funding_limit":
		return appfunding.ErrLimitExceeded
	case "controlled_funding_input":
		return appfunding.ErrInvalidCommand
	default:
		return err
	}
}

func classifyControlledFundingLifecycleError(err error) error {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return fmt.Errorf("execute controlled funding lifecycle command: %w", err)
	}
	constraint := postgresError.ConstraintName
	switch {
	case strings.HasSuffix(constraint, "_actor"), strings.HasSuffix(constraint, "_forbidden"):
		return appfunding.ErrForbidden
	case strings.HasSuffix(constraint, "_not_found"):
		return appfunding.ErrNotFound
	case strings.HasSuffix(constraint, "_limit"):
		return appfunding.ErrLimitExceeded
	case strings.HasSuffix(constraint, "_input"):
		return appfunding.ErrInvalidCommand
	case strings.HasSuffix(constraint, "_idempotency"), strings.HasSuffix(constraint, "_conflict"), postgresError.Code == "23505":
		return appfunding.ErrConflict
	default:
		return err
	}
}

func (r *FundingRepository) Compensate(ctx context.Context, command appfunding.CompensationCommand, fingerprint [sha256.Size]byte) (submission appfunding.Submission, err error) {
	sequence := "funding-compensate|" + strings.ToLower(command.TenantID) + "|" + command.FundingEventID
	err = WithTenantSerializableSequence(ctx, r.database, command.TenantID, sequence, 5, func(tx *sql.Tx) error {
		var eventID string
		var replayed bool
		if err := tx.QueryRowContext(ctx, `SELECT funding_event_id::text,replayed FROM public.controlled_request_funding_compensation_v1($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			command.TenantID, command.ActorSubjectID, command.FundingEventID, command.ReasonCode,
			command.OperatorNote, command.IdempotencyKey, fingerprint[:], command.CorrelationID, command.OccurredAt,
		).Scan(&eventID, &replayed); err != nil {
			return classifyControlledFundingLifecycleError(err)
		}
		event, readErr := readFundingEventByID(ctx, tx, command.TenantID, eventID)
		if readErr != nil {
			return readErr
		}
		submission = appfunding.Submission{Event: event, Replayed: replayed}
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
