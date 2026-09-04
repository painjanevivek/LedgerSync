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
	err = WithTenantSerializableSequence(ctx, r.database, command.TenantID, "transfer-correction|"+strings.ToLower(command.TenantID)+"|"+command.OriginalTransferID, 5, func(tx *sql.Tx) error {
		var id string
		var replayed bool
		if err := tx.QueryRowContext(ctx, `SELECT correction_id::text,replayed FROM public.controlled_request_transfer_correction_v1($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			command.TenantID, command.ActorSubjectID, command.OriginalTransferID, command.ReasonCode,
			command.OperatorNote, command.IdempotencyKey, fingerprint[:], command.CorrelationID,
			nullableTime(command.StepUpAuthenticatedAt), command.RequestedAt,
		).Scan(&id, &replayed); err != nil {
			return classifyControlledCorrectionLifecycleError(err)
		}
		event, err := readCorrectionByID(ctx, tx, command.TenantID, id)
		if err == nil {
			submission = appcorrections.Submission{Event: event, Replayed: replayed}
		}
		return err
	})
	return submission, err
}

func (r *TransferCorrectionRepository) Approve(ctx context.Context, command appcorrections.DecisionCommand) (event appcorrections.Event, err error) {
	err = WithTenantSerializableSequence(ctx, r.database, command.TenantID, "transfer-correction-decision|"+strings.ToLower(command.TenantID)+"|"+command.CorrectionID, 5, func(tx *sql.Tx) error {
		if err := executeControlledCorrectionDecision(ctx, tx, command.TenantID, command.ActorSubjectID, command.CorrectionID, "approve", command.Reason, command.CorrelationID, command.StepUpAuthenticatedAt, command.DecidedAt); err != nil {
			return err
		}
		var err error
		event, err = readCorrectionByID(ctx, tx, command.TenantID, command.CorrectionID)
		return err
	})
	return event, err
}

func (r *TransferCorrectionRepository) Reject(ctx context.Context, command appcorrections.DecisionCommand) (event appcorrections.Event, err error) {
	err = WithTenantSerializableSequence(ctx, r.database, command.TenantID, "transfer-correction-decision|"+strings.ToLower(command.TenantID)+"|"+command.CorrectionID, 5, func(tx *sql.Tx) error {
		if err := executeControlledCorrectionDecision(ctx, tx, command.TenantID, command.ActorSubjectID, command.CorrectionID, "reject", command.Reason, command.CorrelationID, command.StepUpAuthenticatedAt, command.DecidedAt); err != nil {
			return err
		}
		var err error
		event, err = readCorrectionByID(ctx, tx, command.TenantID, command.CorrectionID)
		return err
	})
	return event, err
}

func (r *TransferCorrectionRepository) Cancel(ctx context.Context, command appcorrections.CancelCommand) (event appcorrections.Event, err error) {
	err = WithTenantSerializableSequence(ctx, r.database, command.TenantID, "transfer-correction-decision|"+strings.ToLower(command.TenantID)+"|"+command.CorrectionID, 5, func(tx *sql.Tx) error {
		if err := executeControlledCorrectionDecision(ctx, tx, command.TenantID, command.ActorSubjectID, command.CorrectionID, "cancel", command.Reason, command.CorrelationID, time.Time{}, command.CancelledAt); err != nil {
			return err
		}
		var err error
		event, err = readCorrectionByID(ctx, tx, command.TenantID, command.CorrectionID)
		return err
	})
	return event, err
}

func (r *TransferCorrectionRepository) Post(ctx context.Context, command appcorrections.PostCommand) (submission appcorrections.Submission, err error) {
	err = WithTenantSerializableSequence(ctx, r.database, command.TenantID, "transfer-correction-post|"+strings.ToLower(command.TenantID)+"|"+command.CorrectionID, 5, func(tx *sql.Tx) error {
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
		return err
	}
}

func executeControlledCorrectionDecision(ctx context.Context, tx *sql.Tx, tenantID, actorID, correctionID, action, reason, correlationID string, stepUpAt, decidedAt time.Time) error {
	var status string
	var replayed bool
	if err := tx.QueryRowContext(ctx, `SELECT decision_status,replayed FROM public.controlled_decide_transfer_correction_v1($1,$2,$3,$4,$5,$6,$7,$8)`,
		tenantID, actorID, correctionID, action, reason, correlationID, nullableTime(stepUpAt), decidedAt,
	).Scan(&status, &replayed); err != nil {
		return classifyControlledCorrectionLifecycleError(err)
	}
	_ = status
	_ = replayed
	return nil
}

func classifyControlledCorrectionLifecycleError(err error) error {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return fmt.Errorf("execute controlled correction lifecycle command: %w", err)
	}
	constraint := postgresError.ConstraintName
	switch {
	case strings.HasSuffix(constraint, "_step_up"):
		return appcorrections.ErrStepUpRequired
	case strings.HasSuffix(constraint, "_actor"), strings.HasSuffix(constraint, "_caller"), strings.HasSuffix(constraint, "_forbidden"):
		return appcorrections.ErrForbidden
	case strings.HasSuffix(constraint, "_not_found"):
		return appcorrections.ErrNotFound
	case strings.HasSuffix(constraint, "_input"):
		return appcorrections.ErrInvalidCommand
	case strings.HasSuffix(constraint, "_idempotency"), strings.HasSuffix(constraint, "_conflict"), postgresError.Code == "23505":
		return appcorrections.ErrConflict
	default:
		return err
	}
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
