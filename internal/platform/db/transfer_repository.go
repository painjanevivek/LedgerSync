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

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/account"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/ledger"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	transferdomain "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/transfer"
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

const (
	transferOperation = "transfers.create.v1"
	transferStatusSQL = "pending"
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
	// PostgreSQL UUID input is case-insensitive. Normalize the textual tenant
	// identity as well so equivalent configured UUID spellings cannot split the
	// cross-replica policy sequence into separate advisory-lock keys.
	sequenceTenant := strings.ToLower(strings.TrimSpace(command.TenantID))
	err = WithSerializableSequence(ctx, r.database, "transfer-policy|"+sequenceTenant, 5, func(tx *sql.Tx) error {
		resolved, replay, err := reserveOrReplay(ctx, tx, command, fingerprint)
		if err != nil {
			return err
		}
		if replay {
			submission = transfers.Submission{Result: resolved, Replayed: true}
			return nil
		}

		result, err := r.postOrReject(ctx, tx, command)
		if err != nil {
			return err
		}
		if err := storeOutcome(ctx, tx, command, result); err != nil {
			return err
		}
		submission = transfers.Submission{Result: result}
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

func reserveOrReplay(ctx context.Context, tx *sql.Tx, command transfers.Command, fingerprint [sha256.Size]byte) (transfers.Result, bool, error) {
	const reserve = `
INSERT INTO idempotency_requests (
    tenant_id, actor_subject_id, operation, idempotency_key, request_fingerprint, state, expires_at
) VALUES ($1, $2, $3, $4, $5, 'in_progress', $6)
ON CONFLICT (tenant_id, actor_subject_id, operation, idempotency_key) DO NOTHING
RETURNING request_fingerprint, state, response_body`

	var storedFingerprint []byte
	var state string
	var body []byte
	err := tx.QueryRowContext(ctx, reserve,
		command.TenantID, command.ActorSubjectID, transferOperation, command.IdempotencyKey,
		fingerprint[:], command.OccurredAt.AddDate(0, 0, 30),
	).Scan(&storedFingerprint, &state, &body)
	if err == nil {
		return transfers.Result{}, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return transfers.Result{}, false, fmt.Errorf("reserve idempotency request: %w", err)
	}

	const getForUpdate = `
SELECT request_fingerprint, state, response_body
FROM idempotency_requests
WHERE tenant_id = $1 AND actor_subject_id = $2 AND operation = $3 AND idempotency_key = $4
FOR UPDATE`
	if err := tx.QueryRowContext(ctx, getForUpdate,
		command.TenantID, command.ActorSubjectID, transferOperation, command.IdempotencyKey,
	).Scan(&storedFingerprint, &state, &body); err != nil {
		return transfers.Result{}, false, fmt.Errorf("load idempotency request: %w", err)
	}
	if len(storedFingerprint) != sha256.Size {
		return transfers.Result{}, false, errors.New("stored idempotency fingerprint is malformed")
	}
	var existing [sha256.Size]byte
	copy(existing[:], storedFingerprint)
	resolution, err := transfers.ResolveExisting(&transfers.ExistingRequest{
		Fingerprint: existing,
		State:       transfers.IdempotencyState(state),
	}, fingerprint)
	if err != nil {
		return transfers.Result{}, false, err
	}
	if resolution != transfers.ResolutionReplay || len(body) == 0 {
		return transfers.Result{}, false, transfers.ErrRequestInProgress
	}
	var result transfers.Result
	if err := json.Unmarshal(body, &result); err != nil {
		return transfers.Result{}, false, fmt.Errorf("decode idempotency outcome: %w", err)
	}
	return result, true, nil
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

func (r *TransferRepository) postOrReject(ctx context.Context, tx *sql.Tx, command transfers.Command) (transfers.Result, error) {
	accounts, err := lockAccounts(ctx, tx, command.TenantID, command.DebitAccountID, command.CreditAccountID)
	if err != nil {
		return transfers.Result{}, err
	}
	source := accounts[command.DebitAccountID]
	destination := accounts[command.CreditAccountID]
	if err := validateAccounts(ctx, tx, command, source, destination); err != nil {
		return transfers.Result{}, err
	}
	if err := r.validateTransferPolicy(ctx, tx, command); err != nil {
		return transfers.Result{}, err
	}

	now := command.OccurredAt.UTC()
	transferID, err := newUUID()
	if err != nil {
		return transfers.Result{}, err
	}
	entry, err := transferdomain.New(transferID, command.TenantID, command.ActorSubjectID, command.DebitAccountID, command.CreditAccountID, command.Amount, now)
	if err != nil {
		return transfers.Result{}, err
	}
	if err := createTransfer(ctx, tx, entry); err != nil {
		return transfers.Result{}, err
	}

	if source.AvailableMinor < command.Amount.Minor() {
		if err := entry.Reject("insufficient_funds", now); err != nil {
			return transfers.Result{}, err
		}
		if err := markTransferRejected(ctx, tx, entry); err != nil {
			return transfers.Result{}, err
		}
		if err := insertAuditEvent(ctx, tx, command, entry.ID, transfers.AuditTransferRejected, "failed", now); err != nil {
			return transfers.Result{}, err
		}
		return rejectedResult(entry, command.Amount), nil
	}

	journalID, err := newUUID()
	if err != nil {
		return transfers.Result{}, err
	}
	if err := entry.Post(journalID, now); err != nil {
		return transfers.Result{}, err
	}
	debitID, err := newUUID()
	if err != nil {
		return transfers.Result{}, err
	}
	creditID, err := newUUID()
	if err != nil {
		return transfers.Result{}, err
	}
	debit, err := ledger.NewPosting(debitID, journalID, source.ID, ledger.Debit, command.Amount, now)
	if err != nil {
		return transfers.Result{}, err
	}
	credit, err := ledger.NewPosting(creditID, journalID, destination.ID, ledger.Credit, command.Amount, now)
	if err != nil {
		return transfers.Result{}, err
	}
	if err := ledger.ValidateBalanced([]ledger.Posting{debit, credit}); err != nil {
		return transfers.Result{}, err
	}
	if err := createJournal(ctx, tx, journalID, command.TenantID, entry.ID, now); err != nil {
		return transfers.Result{}, err
	}
	if err := createPosting(ctx, tx, debit); err != nil {
		return transfers.Result{}, err
	}
	if err := createPosting(ctx, tx, credit); err != nil {
		return transfers.Result{}, err
	}

	updatedSource, err := applyBalanceDelta(ctx, tx, source.ID, -command.Amount.Minor(), now)
	if err != nil {
		return transfers.Result{}, err
	}
	updatedDestination, err := applyBalanceDelta(ctx, tx, destination.ID, command.Amount.Minor(), now)
	if err != nil {
		return transfers.Result{}, err
	}
	if err := markTransferPosted(ctx, tx, entry); err != nil {
		return transfers.Result{}, err
	}
	if err := recordTransferVelocity(ctx, tx, entry.ID); err != nil {
		return transfers.Result{}, err
	}
	if err := insertAuditEvent(ctx, tx, command, entry.ID, transfers.AuditTransferPosted, "succeeded", now); err != nil {
		return transfers.Result{}, err
	}
	result := postedResult(entry, updatedSource, updatedDestination)
	if err := enqueueBalanceEvent(ctx, tx, command, entry.ID, updatedSource, now); err != nil {
		return transfers.Result{}, err
	}
	if err := enqueueBalanceEvent(ctx, tx, command, entry.ID, updatedDestination, now); err != nil {
		return transfers.Result{}, err
	}
	return result, nil
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

func validateAccounts(ctx context.Context, tx *sql.Tx, command transfers.Command, source, destination lockedAccount) error {
	if source.Status != account.StatusActive || destination.Status != account.StatusActive {
		return ErrAccountInactive
	}
	if source.Currency != command.Amount.Currency().Code || destination.Currency != command.Amount.Currency().Code {
		return money.ErrCurrencyMismatch
	}
	const ownership = `
SELECT permission
FROM account_owners
WHERE tenant_id = $1 AND account_id = $2 AND subject_id = $3`
	var permission string
	err := tx.QueryRowContext(ctx, ownership, command.TenantID, source.ID, command.ActorSubjectID).Scan(&permission)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotAuthorized
	}
	if err != nil {
		return fmt.Errorf("read debit ownership: %w", err)
	}
	if permission != string(account.PermissionDebit) {
		return ErrNotAuthorized
	}
	var destinationAllowed bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM account_credit_permissions WHERE tenant_id=$1 AND account_id=$2 AND subject_id=$3)`, command.TenantID, destination.ID, command.ActorSubjectID).Scan(&destinationAllowed); err != nil {
		return fmt.Errorf("read destination authorization: %w", err)
	}
	if !destinationAllowed {
		return ErrDestinationNotAuthorized
	}
	return nil
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
	metadata, err := json.Marshal(map[string]string{"denial_code": code, "source_account_id": command.DebitAccountID, "destination_account_id": command.CreditAccountID})
	if err != nil {
		return err
	}
	_, err = r.database.ExecContext(ctx, `INSERT INTO audit_events (id,tenant_id,actor_subject_id,event_type,target_type,outcome,correlation_id,sanitized_metadata,occurred_at) VALUES ($1,$2,$3,'transfer.policy_denied','transfer_request','failed',$4,$5,$6)`, id, command.TenantID, command.ActorSubjectID, correlationID, metadata, command.OccurredAt.UTC())
	return err
}

func createTransfer(ctx context.Context, tx *sql.Tx, entry transferdomain.Transfer) error {
	const statement = `
INSERT INTO transfers (
    id, tenant_id, actor_subject_id, debit_account_id, credit_account_id,
    amount_minor, currency, status, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := tx.ExecContext(ctx, statement, entry.ID, entry.TenantID, entry.ActorID, entry.DebitAccountID, entry.CreditAccountID, entry.Amount.Minor(), entry.Amount.Currency().Code, transferStatusSQL, entry.CreatedAt)
	return wrap("create transfer", err)
}

func createJournal(ctx context.Context, tx *sql.Tx, journalID, tenantID, transferID string, occurredAt time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO journal_transactions (id, tenant_id, transfer_id, occurred_at) VALUES ($1, $2, $3, $4)`, journalID, tenantID, transferID, occurredAt)
	return wrap("create journal", err)
}

func createPosting(ctx context.Context, tx *sql.Tx, posting ledger.Posting) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO ledger_postings (id, journal_transaction_id, account_id, direction, amount_minor, currency, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)`, posting.ID, posting.JournalID, posting.AccountID, posting.Direction, posting.Amount.Minor(), posting.Amount.Currency().Code, posting.OccurredAt)
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

func markTransferPosted(ctx context.Context, tx *sql.Tx, entry transferdomain.Transfer) error {
	result, err := tx.ExecContext(ctx, `
UPDATE transfers
SET status = 'posted', journal_transaction_id = $2, completed_at = $3
WHERE id = $1 AND status = 'pending'`, entry.ID, entry.JournalTransactionID, entry.CompletedAt)
	if err != nil {
		return wrap("mark transfer posted", err)
	}
	return requireOneRow(result, "mark transfer posted")
}

func markTransferRejected(ctx context.Context, tx *sql.Tx, entry transferdomain.Transfer) error {
	result, err := tx.ExecContext(ctx, `
UPDATE transfers
SET status = 'rejected', rejection_code = $2, completed_at = $3
WHERE id = $1 AND status = 'pending'`, entry.ID, entry.RejectionCode, entry.CompletedAt)
	if err != nil {
		return wrap("mark transfer rejected", err)
	}
	return requireOneRow(result, "mark transfer rejected")
}

func insertAuditEvent(ctx context.Context, tx *sql.Tx, command transfers.Command, transferID, eventType, outcome string, now time.Time) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(map[string]string{"transfer_id": transferID})
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}
	correlationID := command.CorrelationID
	if correlationID == "" {
		correlationID, err = newUUID()
		if err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO audit_events (id, tenant_id, actor_subject_id, event_type, target_type, target_id, outcome, correlation_id, sanitized_metadata, occurred_at)
VALUES ($1, $2, $3, $4, 'transfer', $5, $6, $7, $8, $9)`, id, command.TenantID, command.ActorSubjectID, eventType, transferID, outcome, correlationID, metadata, now)
	return wrap("insert audit event", err)
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
INSERT INTO outbox_events (id, tenant_id, transfer_id, account_id, event_type, aggregate_version, payload, occurred_at)
VALUES ($1, $2, $3, $4, 'account.balance.changed.v1', $5, $6, $7)`, id, command.TenantID, transferID, balance.ID, balance.BalanceVersion, payload, now)
	return wrap("enqueue balance event", err)
}

func storeOutcome(ctx context.Context, tx *sql.Tx, command transfers.Command, result transfers.Result) error {
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal idempotency outcome: %w", err)
	}
	updated, err := tx.ExecContext(ctx, `
UPDATE idempotency_requests
SET state = 'completed', response_status = $5, response_body = $6::jsonb, transfer_id = $7, completed_at = $8
WHERE tenant_id = $1 AND actor_subject_id = $2 AND operation = $3 AND idempotency_key = $4
  AND state = 'in_progress'`, command.TenantID, command.ActorSubjectID, transferOperation, command.IdempotencyKey, 201, string(body), result.TransferID, command.OccurredAt)
	if err != nil {
		return wrap("store idempotency outcome", err)
	}
	return requireOneRow(updated, "store idempotency outcome")
}

func postedResult(entry transferdomain.Transfer, source, destination lockedAccount) transfers.Result {
	return transfers.Result{
		TransferID:             entry.ID,
		Status:                 string(transferdomain.StatusPosted),
		Currency:               entry.Amount.Currency().Code,
		AmountMinor:            entry.Amount.Minor(),
		OccurredAt:             entry.CompletedAt.UTC().Format(time.RFC3339Nano),
		MinimumBalanceVersions: map[string]int64{source.ID: source.BalanceVersion, destination.ID: destination.BalanceVersion},
		Balances: map[string]transfers.Balance{
			source.ID:      toBalance(source, entry.Amount.Currency().Code, entry.CompletedAt.UTC()),
			destination.ID: toBalance(destination, entry.Amount.Currency().Code, entry.CompletedAt.UTC()),
		},
	}
}

func rejectedResult(entry transferdomain.Transfer, amount money.Money) transfers.Result {
	return transfers.Result{
		TransferID:             entry.ID,
		Status:                 string(transferdomain.StatusRejected),
		Currency:               amount.Currency().Code,
		AmountMinor:            amount.Minor(),
		OccurredAt:             entry.CompletedAt.UTC().Format(time.RFC3339Nano),
		MinimumBalanceVersions: map[string]int64{},
		RejectionCode:          entry.RejectionCode,
	}
}

func toBalance(account lockedAccount, currency string, occurredAt time.Time) transfers.Balance {
	return transfers.Balance{AccountID: account.ID, Currency: currency, PostedMinor: account.LedgerMinor, Version: account.BalanceVersion, AsOf: occurredAt.Format(time.RFC3339Nano)}
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
