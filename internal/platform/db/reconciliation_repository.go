package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	"go.opentelemetry.io/otel/trace"
)

const reconciliationScope = "tenant_all_accounts"

type ReconciliationRepository struct {
	database  *sql.DB
	telemetry *observability.Telemetry
}

type reconciliationMismatch struct {
	ID, AccountID, TransferID, Classification, Currency string
	ExpectedMinor, ObservedMinor                        *int64
	ObservedAvailableMinor, BalanceVersion              *int64
	Details                                             map[string]any
}

func NewReconciliationRepository(database *sql.DB, telemetry ...*observability.Telemetry) (*ReconciliationRepository, error) {
	if database == nil {
		return nil, errors.New("reconciliation database is required")
	}
	return &ReconciliationRepository{database: database, telemetry: firstTelemetry(telemetry)}, nil
}

// Reconcile compares the complete tenant account population in one repeatable
// read snapshot. Missing rows are mismatches, never exclusions, and both the
// summary and mismatch evidence commit atomically with an audit record.
func (r *ReconciliationRepository) Reconcile(ctx context.Context, tenantID string, startedAt time.Time) (result reconciliation.Result, err error) {
	started := time.Now()
	ctx, span := r.start(ctx, "db.reconciliation.run")
	defer func() { r.observe(ctx, started, err); span.End() }()

	tx, err := beginTenantTransaction(ctx, r.database, TenantScope{TenantID: tenantID, ActorID: "system:reconciliation"}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return reconciliation.Result{}, fmt.Errorf("begin reconciliation snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	locked, err := acquireReconciliationLock(ctx, tx.Tx, tenantID)
	if err != nil {
		return reconciliation.Result{}, err
	}
	if !locked {
		return reconciliation.Result{}, reconciliation.ErrAlreadyRunning
	}
	result, err = r.reconcileTx(ctx, tx.Tx, tenantID, "", "", "", startedAt)
	if err != nil {
		return reconciliation.Result{}, err
	}
	if err := tx.Commit(); err != nil {
		return reconciliation.Result{}, fmt.Errorf("commit reconciliation evidence: %w", err)
	}
	return result, nil
}

func (r *ReconciliationRepository) reconcileTx(ctx context.Context, tx *sql.Tx, tenantID, actorID, correlationID, runID string, startedAt time.Time) (reconciliation.Result, error) {

	var watermark, schemaVersion string
	snapshot := &reconciliation.OutboxSnapshot{Isolation: "repeatable_read"}
	if err := tx.QueryRowContext(ctx, `SELECT transaction_timestamp(),count(*) FILTER(WHERE published_at IS NULL AND dead_at IS NULL)::text,count(*) FILTER(WHERE published_at IS NOT NULL)::text,count(*) FILTER(WHERE dead_at IS NOT NULL)::text FROM outbox_events WHERE tenant_id=$1`, tenantID).Scan(&snapshot.SampledAt, &snapshot.PendingCount, &snapshot.PublishedCount, &snapshot.DeadCount); err != nil {
		return reconciliation.Result{}, fmt.Errorf("read snapshot delivery counts: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT txid_current_snapshot()::text, COALESCE((SELECT max(version) FROM schema_migrations), 'unknown')`).Scan(&watermark, &schemaVersion); err != nil {
		return reconciliation.Result{}, fmt.Errorf("read reconciliation watermark: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `
WITH posting_totals AS (
  SELECT p.account_id,
    SUM(CASE WHEN p.direction='credit' THEN p.amount_minor::numeric ELSE -p.amount_minor::numeric END) AS posted_delta,
    count(*) AS posting_count
  FROM ledger_postings p
  JOIN accounts pa ON pa.id=p.account_id AND pa.tenant_id=p.tenant_id
  WHERE p.tenant_id=$1
  GROUP BY p.account_id
)
SELECT a.id::text,a.currency,
  b.available_minor::text,b.ledger_minor::text,b.balance_version::text,
  o.opening_ledger_minor::text,
  COALESCE(pt.posted_delta,0)::text,COALESCE(pt.posting_count,0)::bigint,
 COALESCE((SELECT sum(h.amount_minor::numeric) FROM account_holds h WHERE h.tenant_id=a.tenant_id AND h.debit_account_id=a.id AND h.state='active'),0)::text
FROM accounts a
LEFT JOIN account_balance_projections b ON b.account_id=a.id
LEFT JOIN account_opening_balances o ON o.account_id=a.id
LEFT JOIN posting_totals pt ON pt.account_id=a.id
WHERE a.tenant_id=$1
ORDER BY a.id`, tenantID)
	if err != nil {
		return reconciliation.Result{}, fmt.Errorf("compare reconciliation scope: %w", err)
	}

	checked, postingCount := 0, 0
	var mismatches []reconciliationMismatch
	for rows.Next() {
		checked++
		var accountID, currency, postedDelta, heldMinor string
		var available, ledger, version, opening sql.NullString
		var accountPostings int64
		if err := rows.Scan(&accountID, &currency, &available, &ledger, &version, &opening, &postedDelta, &accountPostings, &heldMinor); err != nil {
			_ = rows.Close()
			return reconciliation.Result{}, fmt.Errorf("scan reconciliation account: %w", err)
		}
		postingCount += int(accountPostings)
		accountMismatches, err := compareAccount(accountID, currency, available, ledger, version, opening, postedDelta, heldMinor)
		if err != nil {
			_ = rows.Close()
			return reconciliation.Result{}, err
		}
		mismatches = append(mismatches, accountMismatches...)
	}
	if err := rows.Close(); err != nil {
		return reconciliation.Result{}, fmt.Errorf("close reconciliation rows: %w", err)
	}
	if checked == 0 {
		mismatches = append(mismatches, reconciliationMismatch{Classification: "scope_empty", Details: map[string]any{"scope": reconciliationScope}})
	}

	incomplete, err := findIncompleteTransfers(ctx, tx, tenantID)
	if err != nil {
		return reconciliation.Result{}, err
	}
	mismatches = append(mismatches, incomplete...)

	if runID == "" {
		runID, err = newUUID()
		if err != nil {
			return reconciliation.Result{}, err
		}
	}
	if correlationID == "" {
		correlationID = runID
	}
	completedAt := time.Now().UTC()
	status := reconciliation.StatusMatched
	if len(mismatches) > 0 {
		status = reconciliation.StatusMismatch
	}
	applicationVersion := buildVersion()
	details, _ := json.Marshal(map[string]any{"comparison": "opening_baseline_plus_immutable_postings_to_projection", "scope": reconciliationScope, "ledger_watermark": watermark, "application_version": applicationVersion, "schema_version": schemaVersion, "posting_count": strconv.Itoa(postingCount), "outbox_snapshot": snapshot})
	if mismatches == nil {
		mismatches = []reconciliationMismatch{}
	}
	encodedMismatches, err := json.Marshal(mismatches)
	if err != nil {
		return reconciliation.Result{}, fmt.Errorf("encode reconciliation mismatches: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `SELECT public.controlled_record_reconciliation_v1($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, runID, tenantID, nullableString(actorID), checked, postingCount, correlationID, startedAt, completedAt, details, watermark, applicationVersion, schemaVersion, encodedMismatches); err != nil {
		return reconciliation.Result{}, fmt.Errorf("persist controlled reconciliation evidence: %w", err)
	}
	return reconciliation.Result{OutboxSnapshot: snapshot, ID: runID, TenantID: tenantID, CorrelationID: correlationID, Scope: reconciliationScope, LedgerWatermark: watermark, ApplicationVersion: applicationVersion, SchemaVersion: schemaVersion, Status: status, CheckedAccountCount: checked, PostingCount: postingCount, MismatchCount: len(mismatches), StartedAt: startedAt, CompletedAt: completedAt}, nil
}

func acquireReconciliationLock(ctx context.Context, tx *sql.Tx, tenantID string) (bool, error) {
	var locked bool
	if err := tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1, 824631))`, tenantID).Scan(&locked); err != nil {
		return false, fmt.Errorf("acquire tenant reconciliation lock: %w", err)
	}
	return locked, nil
}

func compareAccount(accountID, currency string, available, ledger, version, opening sql.NullString, postedDelta string, heldMinor ...string) ([]reconciliationMismatch, error) {
	if !ledger.Valid || !available.Valid || !version.Valid {
		return []reconciliationMismatch{{AccountID: accountID, Currency: currency, Classification: "projection_missing"}}, nil
	}
	observedLedger, err := parseDatabaseInt(ledger.String)
	if err != nil {
		return nil, fmt.Errorf("parse observed ledger for %s: %w", accountID, err)
	}
	observedAvailable, err := parseDatabaseInt(available.String)
	if err != nil {
		return nil, fmt.Errorf("parse observed available for %s: %w", accountID, err)
	}
	balanceVersion, err := parseDatabaseInt(version.String)
	if err != nil {
		return nil, fmt.Errorf("parse balance version for %s: %w", accountID, err)
	}
	if !opening.Valid {
		return []reconciliationMismatch{{AccountID: accountID, Currency: currency, Classification: "opening_balance_missing", ObservedMinor: &observedLedger, ObservedAvailableMinor: &observedAvailable, BalanceVersion: &balanceVersion}}, nil
	}
	expectedBig, ok := new(big.Int).SetString(opening.String, 10)
	if !ok {
		return nil, fmt.Errorf("parse opening balance for %s", accountID)
	}
	deltaBig, ok := new(big.Int).SetString(postedDelta, 10)
	if !ok {
		return nil, fmt.Errorf("parse posted delta for %s", accountID)
	}
	expectedBig.Add(expectedBig, deltaBig)
	var result []reconciliationMismatch
	if !expectedBig.IsInt64() || expectedBig.Int64() != observedLedger {
		mismatch := reconciliationMismatch{AccountID: accountID, Currency: currency, Classification: "ledger_balance_mismatch", ObservedMinor: &observedLedger, ObservedAvailableMinor: &observedAvailable, BalanceVersion: &balanceVersion, Details: map[string]any{"expected_numeric": expectedBig.String()}}
		if expectedBig.IsInt64() {
			expected := expectedBig.Int64()
			mismatch.ExpectedMinor = &expected
		}
		result = append(result, mismatch)
	}
	reserved := new(big.Int)
	if len(heldMinor) > 0 {
		if _, ok := reserved.SetString(heldMinor[0], 10); !ok || reserved.Sign() < 0 {
			return nil, fmt.Errorf("invalid reservation obligation for %s", accountID)
		}
	}
	expectedAvailable := new(big.Int).Sub(big.NewInt(observedLedger), reserved)
	if !expectedAvailable.IsInt64() || expectedAvailable.Int64() != observedAvailable {
		mismatch := reconciliationMismatch{AccountID: accountID, Currency: currency, Classification: "available_balance_mismatch", ObservedMinor: &observedLedger, ObservedAvailableMinor: &observedAvailable, BalanceVersion: &balanceVersion, Details: map[string]any{"expected_numeric": expectedAvailable.String()}}
		if expectedAvailable.IsInt64() {
			expected := expectedAvailable.Int64()
			mismatch.ExpectedMinor = &expected
		}
		result = append(result, mismatch)
	}
	return result, nil
}

func findIncompleteTransfers(ctx context.Context, tx *sql.Tx, tenantID string) ([]reconciliationMismatch, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT t.id::text,COALESCE(t.journal_transaction_id::text,''),count(p.id),
  COALESCE(SUM(CASE WHEN p.direction='credit' THEN p.amount_minor::numeric ELSE -p.amount_minor::numeric END),0)::text
FROM transfers t
LEFT JOIN journal_transactions j ON j.id=t.journal_transaction_id AND j.tenant_id=t.tenant_id
LEFT JOIN ledger_postings p ON p.journal_transaction_id=j.id AND p.tenant_id=j.tenant_id
WHERE t.tenant_id=$1 AND t.status='posted'
GROUP BY t.id,t.journal_transaction_id
HAVING count(p.id)<2 OR COALESCE(SUM(CASE WHEN p.direction='credit' THEN p.amount_minor::numeric ELSE -p.amount_minor::numeric END),0)<>0`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("check posted transfer evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var result []reconciliationMismatch
	for rows.Next() {
		var transferID, journalID, signedTotal string
		var count int
		if err := rows.Scan(&transferID, &journalID, &count, &signedTotal); err != nil {
			return nil, err
		}
		classification := "posted_transfer_incomplete"
		if count >= 2 && signedTotal != "0" {
			classification = "journal_unbalanced"
		}
		result = append(result, reconciliationMismatch{TransferID: transferID, Classification: classification, Details: map[string]any{"transfer_id": transferID, "journal_transaction_id": journalID, "posting_count": count, "signed_total": signedTotal}})
	}
	return result, rows.Err()
}

func nullableUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func parseDatabaseInt(value string) (int64, error) { return strconv.ParseInt(value, 10, 64) }

func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "development"
}

func (r *ReconciliationRepository) start(ctx context.Context, name string) (context.Context, trace.Span) {
	return r.telemetry.Start(ctx, name)
}

func (r *ReconciliationRepository) observe(ctx context.Context, started time.Time, err error) {
	if r.telemetry != nil {
		r.telemetry.ObserveBoundary(ctx, "postgres", "reconciliation", started, err)
	}
}
