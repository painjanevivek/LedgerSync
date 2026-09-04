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
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/account"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/ledger"
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

type lockedAccount struct {
	ID             string
	TenantID       string
	Currency       string
	Status         account.Status
	AvailableMinor int64
	LedgerMinor    int64
	BalanceVersion int64
}

func lockAccounts(ctx context.Context, tx *sql.Tx, tenantID, sourceID, destinationID string) (map[string]lockedAccount, error) {
	const query = `
SELECT a.id, a.tenant_id, a.currency, a.status, b.available_minor, b.ledger_minor, b.balance_version
FROM accounts AS a
JOIN account_balance_projections AS b ON b.account_id = a.id
WHERE a.tenant_id = $1 AND (a.id = $2 OR a.id = $3)
ORDER BY a.id
FOR UPDATE OF a, b`
	rows, err := tx.QueryContext(ctx, query, tenantID, sourceID, destinationID)
	if err != nil {
		return nil, fmt.Errorf("lock account projections: %w", err)
	}
	defer func() { _ = rows.Close() }()
	accounts := make(map[string]lockedAccount, 2)
	for rows.Next() {
		var item lockedAccount
		if err := rows.Scan(&item.ID, &item.TenantID, &item.Currency, &item.Status, &item.AvailableMinor, &item.LedgerMinor, &item.BalanceVersion); err != nil {
			return nil, fmt.Errorf("scan locked account: %w", err)
		}
		accounts[item.ID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate locked accounts: %w", err)
	}
	if len(accounts) != 2 {
		return nil, ErrAccountNotFound
	}
	return accounts, nil
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
	_, err = r.database.ExecContext(ctx, `INSERT INTO audit_events (id,tenant_id,actor_subject_id,event_type,target_type,outcome,correlation_id,sanitized_metadata,occurred_at) VALUES ($1,$2,$3,'transfer.policy_denied','transfer_request','failed',$4,$5,$6)`, id, command.TenantID, command.ActorSubjectID, correlationID, metadata, command.OccurredAt.UTC())
	return err
}

func createJournal(ctx context.Context, tx *sql.Tx, journalID, tenantID, transferID string, occurredAt time.Time) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO journal_transactions (id, tenant_id, transfer_id, source_type, source_id, occurred_at)
VALUES ($1, $2, $3, 'transfer', $3, $4)`, journalID, tenantID, transferID, occurredAt)
	return wrap("create journal", err)
}

func createPosting(ctx context.Context, tx *sql.Tx, tenantID string, posting ledger.Posting) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO ledger_postings (id, journal_transaction_id, tenant_id, account_id, direction, amount_minor, currency, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, posting.ID, posting.JournalID, tenantID, posting.AccountID, posting.Direction, posting.Amount.Minor(), posting.Amount.Currency().Code, posting.OccurredAt)
	return wrap("create ledger posting", err)
}

func applyBalanceDelta(ctx context.Context, tx *sql.Tx, accountID string, delta int64, now time.Time) (lockedAccount, error) {
	const statement = `
UPDATE account_balance_projections
SET available_minor = available_minor + $2,
    ledger_minor = ledger_minor + $2,
    balance_version = balance_version + 1,
    updated_at = $3
WHERE account_id = $1
  AND available_minor + $2 >= 0
  AND ledger_minor + $2 >= 0
RETURNING available_minor, ledger_minor, balance_version, updated_at`
	item := lockedAccount{ID: accountID}
	err := tx.QueryRowContext(ctx, statement, accountID, delta, now).Scan(&item.AvailableMinor, &item.LedgerMinor, &item.BalanceVersion, new(time.Time))
	if errors.Is(err, sql.ErrNoRows) {
		return lockedAccount{}, ErrInsufficientFunds
	}
	if err != nil {
		return lockedAccount{}, fmt.Errorf("apply balance delta: %w", err)
	}
	return item, nil
}

func enqueueBalanceEvent(ctx context.Context, tx *sql.Tx, command transfers.Command, transferID string, balance lockedAccount, now time.Time) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"event_id":        id,
		"event_type":      "account.balance.changed.v1",
		"account_id":      balance.ID,
		"transfer_id":     transferID,
		"currency":        command.Amount.Currency().Code,
		"available_minor": strconv.FormatInt(balance.AvailableMinor, 10),
		"balance_version": strconv.FormatInt(balance.BalanceVersion, 10),
		"occurred_at":     now.Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("marshal outbox event: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO outbox_events (id, tenant_id, transfer_id, account_id, aggregate_type, aggregate_id, event_type, aggregate_version, payload, occurred_at)
VALUES ($1, $2, $3, $4, 'account_balance', $4, 'account.balance.changed.v1', $5, $6, $7)`, id, command.TenantID, transferID, balance.ID, balance.BalanceVersion, payload, now)
	return wrap("enqueue balance event", err)
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

func wrap(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
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
