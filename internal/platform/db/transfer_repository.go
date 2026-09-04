package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrAccountNotFound          = errors.New("transfer account not found")
	ErrNotAuthorized            = errors.New("transfer source account is not authorized")
	ErrAccountInactive          = errors.New("transfer account is not active")
	ErrInsufficientFunds        = errors.New("insufficient funds")
	ErrDestinationNotAuthorized = errors.New("transfer destination account is not authorized")
	ErrTenantPolicyMissing      = errors.New("tenant transfer policy is missing")
	ErrTransferBelowMinimum     = errors.New("transfer amount is below tenant minimum")
	ErrTransferAboveMaximum     = errors.New("transfer amount exceeds tenant maximum")
	ErrActorVelocityExceeded    = errors.New("actor rolling transfer limit exceeded")
	ErrSourceVelocityExceeded   = errors.New("source account rolling transfer limit exceeded")
	ErrTenantVelocityExceeded   = errors.New("tenant rolling transfer limit exceeded")
	ErrUnsupportedPilotCurrency = errors.New("transfer currency is outside the configured pilot")
)

// TransferRepository is the PostgreSQL financial command adapter. All writes
// in Submit happen inside one serializable transaction and no external call is
// made before the commit succeeds.
type TransferRepository struct {
	database      *sql.DB
	clock         func() time.Time
	telemetry     *observability.Telemetry
	pilotCurrency string
}

func NewTransferRepository(database *sql.DB, clock func() time.Time, telemetry ...*observability.Telemetry) (*TransferRepository, error) {
	if database == nil {
		return nil, errors.New("transfer database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &TransferRepository{database: database, clock: clock, telemetry: firstTelemetry(telemetry), pilotCurrency: "USD"}, nil
}

func (r *TransferRepository) WithPilotCurrency(currency string) *TransferRepository {
	r.pilotCurrency = strings.ToUpper(strings.TrimSpace(currency))
	return r
}

func (r *TransferRepository) Submit(ctx context.Context, command transfers.Command, fingerprint [sha256.Size]byte) (submission transfers.Submission, err error) {
	started := time.Now()
	ctx, span := r.start(ctx, "db.transfer.submit")
	defer func() { span.End(); r.observe(ctx, "transfer_submit", started, err) }()
	if command.OccurredAt.IsZero() {
		command.OccurredAt = r.clock().UTC()
	}
	if command.CorrelationID == "" {
		command.CorrelationID, err = newUUID()
		if err != nil {
			return transfers.Submission{}, err
		}
	}
	sequenceTenant := command.TenantID.String()
	err = WithSerializableSequence(ctx, r.database, "transfer-policy|"+sequenceTenant, 5, func(tx *sql.Tx) error {
		var response []byte
		var replayed bool
		queryErr := tx.QueryRowContext(ctx, `
SELECT response_body,replayed
FROM public.controlled_submit_transfer_v1($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			command.TenantID, command.ActorSubjectID, command.DebitAccountID, command.CreditAccountID,
			command.Amount.Minor(), command.Amount.Currency().Code, command.IdempotencyKey, fingerprint[:], command.CorrelationID,
			r.pilotCurrency, command.OccurredAt,
		).Scan(&response, &replayed)
		if queryErr != nil {
			return classifyControlledTransferError(queryErr)
		}
		var result transfers.Result
		if unmarshalErr := json.Unmarshal(response, &result); unmarshalErr != nil {
			return fmt.Errorf("decode controlled transfer outcome: %w", unmarshalErr)
		}
		submission = transfers.Submission{Result: result, Replayed: replayed}
		return nil
	})
	if err != nil {
		if code := policyDenialCode(err); code != "" {
			if auditErr := r.recordDeniedAudit(ctx, command, code); auditErr != nil {
				return transfers.Submission{}, fmt.Errorf("persist required transfer denial audit: %w", auditErr)
			}
		}
	}
	return submission, err
}

func classifyControlledTransferError(err error) error {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return fmt.Errorf("execute controlled transfer: %w", err)
	}
	switch postgresError.ConstraintName {
	case "controlled_transfer_actor", "controlled_transfer_source_actor":
		return ErrNotAuthorized
	case "controlled_transfer_destination_actor":
		return ErrDestinationNotAuthorized
	case "controlled_transfer_tenant":
		return ErrAccountNotFound
	case "controlled_transfer_account_status":
		return ErrAccountInactive
	case "controlled_transfer_policy_missing":
		return ErrTenantPolicyMissing
	case "controlled_transfer_currency":
		return ErrUnsupportedPilotCurrency
	case "controlled_transfer_account_currency":
		return money.ErrCurrencyMismatch
	case "controlled_transfer_amount_minimum":
		return ErrTransferBelowMinimum
	case "controlled_transfer_amount_maximum":
		return ErrTransferAboveMaximum
	case "controlled_transfer_actor_velocity":
		return ErrActorVelocityExceeded
	case "controlled_transfer_source_velocity":
		return ErrSourceVelocityExceeded
	case "controlled_transfer_tenant_velocity":
		return ErrTenantVelocityExceeded
	case "controlled_transfer_idempotency":
		return transfers.ErrIdempotencyConflict
	case "controlled_transfer_in_progress":
		return transfers.ErrRequestInProgress
	case "controlled_transfer_input", "controlled_transfer_fingerprint":
		return transfers.ErrInvalidCommand
	default:
		return err
	}
}

func (r *TransferRepository) start(ctx context.Context, name string) (context.Context, trace.Span) {
	if r.telemetry == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return r.telemetry.Start(ctx, name)
}

func (r *TransferRepository) observe(ctx context.Context, operation string, started time.Time, err error) {
	if r.telemetry != nil {
		r.telemetry.ObserveBoundary(ctx, "postgres", operation, started, err)
	}
}

func policyDenialCode(err error) string {
	switch {
	case errors.Is(err, ErrNotAuthorized):
		return "source_not_authorized"
	case errors.Is(err, ErrDestinationNotAuthorized):
		return "destination_not_authorized"
	case errors.Is(err, ErrTenantPolicyMissing):
		return "tenant_policy_missing"
	case errors.Is(err, ErrTransferBelowMinimum):
		return "amount_below_minimum"
	case errors.Is(err, ErrTransferAboveMaximum):
		return "amount_above_maximum"
	case errors.Is(err, ErrActorVelocityExceeded):
		return "actor_velocity_exceeded"
	case errors.Is(err, ErrSourceVelocityExceeded):
		return "source_velocity_exceeded"
	case errors.Is(err, ErrTenantVelocityExceeded):
		return "tenant_velocity_exceeded"
	case errors.Is(err, ErrUnsupportedPilotCurrency):
		return "unsupported_pilot_currency"
	default:
		return ""
	}
}

func (r *TransferRepository) recordDeniedAudit(ctx context.Context, command transfers.Command, code string) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	correlationID := command.CorrelationID
	if correlationID == "" {
		correlationID, err = newUUID()
		if err != nil {
			return err
		}
	}
	metadata, err := json.Marshal(map[string]string{"denial_code": code, "source_account_id": command.DebitAccountID.String(), "destination_account_id": command.CreditAccountID.String()})
	if err != nil {
		return err
	}
	return appendControlledAuditPayload(ctx, r.database, id, AuditEvent{
		TenantID: command.TenantID.String(), ActorSubjectID: command.ActorSubjectID,
		EventType: "transfer.policy_denied", TargetType: "transfer_request", Outcome: "failed",
		CorrelationID: correlationID, OccurredAt: command.OccurredAt.UTC(),
	}, metadata)
}

func newUUID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate UUID: %w", err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw[:])
	return strings.Join([]string{encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]}, "-"), nil
}

func requireOneRow(result sql.Result, operation string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", operation, err)
	}
	if rows != 1 {
		return fmt.Errorf("%s affected %d rows", operation, rows)
	}
	return nil
}
