package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
)

type ReconciliationRepository struct{ database *sql.DB }

func NewReconciliationRepository(database *sql.DB) (*ReconciliationRepository, error) {
	if database == nil {
		return nil, errors.New("reconciliation database is required")
	}
	return &ReconciliationRepository{database}, nil
}

// Reconcile compares each current projection with the newest committed balance
// event for that account. Accounts without a transfer event are intentionally
// excluded: legacy opening balances cannot be reconstructed from no ledger data.
func (r *ReconciliationRepository) Reconcile(ctx context.Context, tenantID string, startedAt time.Time) (reconciliation.Result, error) {
	const query = `
WITH latest_event AS (
  SELECT DISTINCT ON (account_id) account_id, aggregate_version, (payload->>'available_minor')::bigint AS available_minor
  FROM outbox_events WHERE tenant_id=$1 AND event_type='account.balance.changed.v1'
  ORDER BY account_id, aggregate_version DESC, occurred_at DESC
), comparison AS (
  SELECT p.account_id, (p.available_minor = e.available_minor AND p.balance_version = e.aggregate_version) AS matches
  FROM latest_event e JOIN account_balance_projections p ON p.account_id=e.account_id
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
	details, _ := json.Marshal(map[string]any{"comparison": "latest_outbox_event_to_projection"})
	if _, err = r.database.ExecContext(ctx, `INSERT INTO reconciliation_runs (id,tenant_id,status,checked_account_count,mismatch_count,correlation_id,started_at,completed_at,details) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, tenantID, status, checked, mismatches, id, startedAt, completed, details); err != nil {
		return reconciliation.Result{}, fmt.Errorf("persist reconciliation result: %w", err)
	}
	return reconciliation.Result{ID: id, TenantID: tenantID, Status: status, CheckedAccountCount: checked, MismatchCount: mismatches, StartedAt: startedAt, CompletedAt: completed}, nil
}
