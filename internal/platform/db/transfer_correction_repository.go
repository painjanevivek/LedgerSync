package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	appcorrections "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/corrections"
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
	err = WithTenantSerializableSequence(ctx, r.database, TenantScope{TenantID: command.TenantID, ActorID: command.ActorSubjectID}, "transfer-correction|"+strings.ToLower(command.TenantID)+"|"+command.OriginalTransferID, 5, func(tx *sql.Tx) error {
		var correctionID string
		var replayed bool
		if err := tx.QueryRowContext(ctx, `SELECT correction_id,replayed FROM public.controlled_request_correction_v1($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, command.TenantID, command.ActorSubjectID, command.OriginalTransferID, command.ReasonCode, command.OperatorNote, command.IdempotencyKey, fingerprint[:], command.CorrelationID, command.RequestedAt, nullableTime(command.StepUpAuthenticatedAt)).Scan(&correctionID, &replayed); err != nil {
			return classifyControlledCorrectionError(err)
		}
		event, err := readCorrectionByID(ctx, tx, command.TenantID, correctionID)
		submission = appcorrections.Submission{Event: event, Replayed: replayed}
		return err
	})
	return submission, err
}

func (r *TransferCorrectionRepository) Approve(ctx context.Context, command appcorrections.DecisionCommand) (appcorrections.Event, error) {
	return r.decide(ctx, command, "approved")
}

func (r *TransferCorrectionRepository) Reject(ctx context.Context, command appcorrections.DecisionCommand) (appcorrections.Event, error) {
	return r.decide(ctx, command, "rejected")
}

func (r *TransferCorrectionRepository) Cancel(ctx context.Context, command appcorrections.CancelCommand) (appcorrections.Event, error) {
	return r.decide(ctx, appcorrections.DecisionCommand{TenantID: command.TenantID, ActorSubjectID: command.ActorSubjectID, CorrectionID: command.CorrectionID, Reason: command.Reason, CorrelationID: command.CorrelationID, DecidedAt: command.CancelledAt}, "cancelled")
}

func (r *TransferCorrectionRepository) decide(ctx context.Context, command appcorrections.DecisionCommand, decision string) (event appcorrections.Event, err error) {
	err = WithTenantSerializableSequence(ctx, r.database, TenantScope{TenantID: command.TenantID, ActorID: command.ActorSubjectID}, "transfer-correction-decision|"+strings.ToLower(command.TenantID)+"|"+command.CorrectionID, 5, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `SELECT public.controlled_decide_correction_v1($1,$2,$3,$4,$5,$6,$7,$8)`, command.TenantID, command.ActorSubjectID, command.CorrectionID, decision, command.Reason, command.CorrelationID, command.DecidedAt, nullableTime(command.StepUpAuthenticatedAt)); err != nil {
			return classifyControlledCorrectionError(err)
		}
		var readErr error
		event, readErr = readCorrectionByID(ctx, tx, command.TenantID, command.CorrectionID)
		return readErr
	})
	return event, err
}

func (r *TransferCorrectionRepository) Post(ctx context.Context, command appcorrections.PostCommand) (submission appcorrections.Submission, err error) {
	err = WithTenantSerializableSequence(ctx, r.database, TenantScope{TenantID: command.TenantID, ActorID: command.ActorSubjectID}, "transfer-correction-post|"+strings.ToLower(command.TenantID)+"|"+command.CorrectionID, 5, func(tx *sql.Tx) error {
		var replayed bool
		if queryErr := tx.QueryRowContext(ctx, `SELECT replayed FROM public.controlled_post_transfer_correction_v1($1,$2,$3,$4,$5,$6,$7)`,
			command.TenantID, command.ActorSubjectID, command.CorrectionID,
			command.IdempotencyKey, command.CorrelationID, command.OccurredAt,
			nullableTime(command.StepUpAuthenticatedAt),
		).Scan(&replayed); queryErr != nil {
			return classifyControlledCorrectionError(queryErr)
		}
		event, readErr := readCorrectionByID(ctx, tx, command.TenantID, command.CorrectionID)
		if readErr != nil {
			return readErr
		}
		submission = appcorrections.Submission{Event: event, Replayed: replayed}
		return nil
	})
	return submission, err
}

func classifyControlledCorrectionError(err error) error {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return fmt.Errorf("execute controlled correction post: %w", err)
	}
	switch postgresError.ConstraintName {
	case "controlled_correction_actor", "controlled_correction_forbidden":
		return appcorrections.ErrForbidden
	case "controlled_correction_not_found":
		return appcorrections.ErrNotFound
	case "controlled_correction_idempotency", "controlled_correction_conflict":
		return appcorrections.ErrConflict
	case "controlled_correction_step_up":
		return appcorrections.ErrStepUpRequired
	case "controlled_correction_input":
		return appcorrections.ErrInvalidCommand
	default:
		if postgresError.Code == "23505" {
			return appcorrections.ErrConflict
		}
		return err
	}
}

func (r *TransferCorrectionRepository) Get(ctx context.Context, tenantID, actorID, correctionID string) (appcorrections.Event, error) {
	tx, scopeErr := beginTenantRead(ctx, r.database, TenantScope{TenantID: tenantID, ActorID: actorID})
	if scopeErr != nil {
		return appcorrections.Event{}, scopeErr
	}
	defer func() { _ = tx.Rollback() }()
	if err := authorizeCorrectionReaderDB(ctx, tx, tenantID, actorID); err != nil {
		return appcorrections.Event{}, err
	}
	return readCorrectionByID(ctx, tx, tenantID, correctionID)
}
func (r *TransferCorrectionRepository) List(ctx context.Context, tenantID, actorID string, query appcorrections.Query) (appcorrections.Page, error) {
	tx, scopeErr := beginTenantRead(ctx, r.database, TenantScope{TenantID: tenantID, ActorID: actorID})
	if scopeErr != nil {
		return appcorrections.Page{}, scopeErr
	}
	defer func() { _ = tx.Rollback() }()
	if err := authorizeCorrectionReaderDB(ctx, tx, tenantID, actorID); err != nil {
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
	rows, err := tx.QueryContext(ctx, `SELECT `+correctionColumns+` FROM transfer_corrections c JOIN transfers original ON original.id=c.original_transfer_id LEFT JOIN transfers compensation ON compensation.id=c.compensation_transfer_id WHERE c.tenant_id=$1 AND ($2='' OR c.status=$2) AND ($3::timestamptz IS NULL OR (c.requested_at,c.id)<($3::timestamptz,$4::uuid)) ORDER BY c.requested_at DESC,c.id DESC LIMIT $5`, tenantID, query.Status, nullableTime(before), nullableString(beforeID), query.Limit+1)
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

func authorizeCorrectionReaderDB(ctx context.Context, database scopedReadExecutor, tenantID, actorID string) error {
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
