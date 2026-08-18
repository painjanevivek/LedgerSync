package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	"go.opentelemetry.io/otel/trace"
)

type ReconciliationRepository struct {
	database  *sql.DB
	telemetry *observability.Telemetry
}

func NewReconciliationRepository(database *sql.DB, telemetry ...*observability.Telemetry) (*ReconciliationRepository, error) {
	if database == nil {
		return nil, errors.New("reconciliation database is required")
	}
	return &ReconciliationRepository{database: database, telemetry: firstTelemetry(telemetry)}, nil
}

// Reconcile independently recomputes a tenant's posted ledger balance from its
// immutable opening baseline and its immutable debit/credit postings. A missing
// opening baseline is itself a mismatch: reporting a false match is worse than
// asking an operator to establish a reviewed baseline for a migrated account.
func (r *ReconciliationRepository) Reconcile(ctx context.Context, tenantID string, startedAt time.Time) (result reconciliation.Result, err error) {
	started := time.Now()
	ctx, span := r.start(ctx, "db.reconciliation.run")
	defer func() { span.End(); r.observe(ctx, started, err) }()
	const query = `
WITH ledger_totals AS (
  SELECT
    a.id AS account_id,
    b.available_minor,
    b.ledger_minor,
    o.opening_ledger_minor,
    COALESCE(SUM(CASE WHEN p.direction = 'credit' THEN p.amount_minor ELSE -p.amount_minor END), 0) AS posted_delta_minor
  FROM accounts a
  JOIN account_balance_projections b ON b.account_id = a.id
  LEFT JOIN account_opening_balances o ON o.account_id = a.id
  LEFT JOIN ledger_postings p ON p.account_id = a.id
  WHERE a.tenant_id = $1
  GROUP BY a.id, b.available_minor, b.ledger_minor, o.opening_ledger_minor
), comparison AS (
  SELECT account_id,
    opening_ledger_minor IS NOT NULL
      AND ledger_minor = opening_ledger_minor + posted_delta_minor
      AND available_minor = ledger_minor AS matches
  FROM ledger_totals
)
SELECT count(*), count(*) FILTER (WHERE NOT matches) FROM comparison`
	var checked, mismatches int
	if err := r.database.QueryRowContext(ctx, query, tenantID).Scan(&checked, &mismatches); err != nil {
		return reconciliation.Result{}, fmt.Errorf("compare projections: %w", err)
	}
	status := reconciliation.StatusMatched
	if mismatches > 0 {
		status = reconciliation.StatusMismatch
	}
	id, err := newUUID()
	if err != nil {
		return reconciliation.Result{}, err
	}
	completed := time.Now().UTC()
	details, _ := json.Marshal(map[string]any{"comparison": "opening_baseline_plus_immutable_ledger_to_projection"})
	if _, err = r.database.ExecContext(ctx, `INSERT INTO reconciliation_runs (id,tenant_id,status,checked_account_count,mismatch_count,correlation_id,started_at,completed_at,details) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, tenantID, status, checked, mismatches, id, startedAt, completed, details); err != nil {
		return reconciliation.Result{}, fmt.Errorf("persist reconciliation result: %w", err)
	}
	return reconciliation.Result{ID: id, TenantID: tenantID, Status: status, CheckedAccountCount: checked, MismatchCount: mismatches, StartedAt: startedAt, CompletedAt: completed}, nil
}

func (r *ReconciliationRepository) start(ctx context.Context, name string) (context.Context, trace.Span) {
	if r.telemetry == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return r.telemetry.Start(ctx, name)
}

func (r *ReconciliationRepository) observe(ctx context.Context, started time.Time, err error) {
	if r.telemetry != nil {
		r.telemetry.ObserveBoundary(ctx, "postgres", "reconciliation", started, err)
	}
}
