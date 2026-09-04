package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/retention"
)

type RetentionRepository struct {
	database *sql.DB
	clock    func() time.Time
}

func NewRetentionRepository(database *sql.DB, clock func() time.Time) (*RetentionRepository, error) {
	if database == nil {
		return nil, errors.New("retention database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &RetentionRepository{database: database, clock: clock}, nil
}

func (r *RetentionRepository) Run(ctx context.Context, policy retention.Policy, apply bool, correlationID string) (retention.Result, error) {
	if policy.TenantID == "" || policy.BatchSize < 1 || policy.BatchSize > 10_000 || policy.PublishedOutboxAfter < 24*time.Hour || policy.RateWindowAfter < time.Hour || correlationID == "" {
		return retention.Result{}, errors.New("valid tenant, bounded batch, conservative cutoffs, and correlation ID are required")
	}
	now := r.clock().UTC()
	mode := "dry_run"
	if apply {
		mode = "apply"
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return retention.Result{}, err
	}
	defer func() { _ = tx.Rollback() }()
	result := retention.Result{Mode: mode, CorrelationID: correlationID, StartedAt: now}
	if apply {
		result.PublishedOutbox, err = deleteBatch(ctx, tx, `DELETE FROM outbox_events WHERE id IN (SELECT o.id FROM outbox_events o WHERE o.tenant_id=$1 AND o.published_at<$2 AND o.dead_at IS NULL AND NOT EXISTS(SELECT 1 FROM delivery_attempts d WHERE d.outbox_event_id=o.id) AND NOT EXISTS(SELECT 1 FROM webhook_delivery_jobs j WHERE j.outbox_event_id=o.id) AND NOT EXISTS(SELECT 1 FROM outbox_replay_actions a WHERE a.event_id=o.id) ORDER BY o.published_at,o.id LIMIT $3 FOR UPDATE SKIP LOCKED)`, policy.TenantID, now.Add(-policy.PublishedOutboxAfter), policy.BatchSize)
		if err == nil {
			result.ExpiredRates, err = deleteBatch(ctx, tx, `DELETE FROM api_rate_limit_windows WHERE (tenant_id,principal_hash,route_key,window_started_at) IN (SELECT tenant_id,principal_hash,route_key,window_started_at FROM api_rate_limit_windows WHERE tenant_id=$1 AND window_started_at<$2 ORDER BY window_started_at LIMIT $3 FOR UPDATE SKIP LOCKED)`, policy.TenantID, now.Add(-policy.RateWindowAfter), policy.BatchSize)
		}
	}
	if err == nil {
		// Final idempotency outcomes remain immutable after their client retry
		// window. Counting them makes their growth visible without reopening a
		// duplicate-money path by deleting a previously used key.
		err = tx.QueryRowContext(ctx, `SELECT count(*) FROM idempotency_requests WHERE tenant_id=$1 AND state='completed' AND expires_at<$2`, policy.TenantID, now).Scan(&result.RetainedIdempotency)
	}
	if !apply && err == nil {
		err = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM outbox_events o WHERE o.tenant_id=$1 AND o.published_at<$2 AND o.dead_at IS NULL AND NOT EXISTS(SELECT 1 FROM delivery_attempts d WHERE d.outbox_event_id=o.id) AND NOT EXISTS(SELECT 1 FROM webhook_delivery_jobs j WHERE j.outbox_event_id=o.id) AND NOT EXISTS(SELECT 1 FROM outbox_replay_actions a WHERE a.event_id=o.id)),(SELECT count(*) FROM api_rate_limit_windows WHERE tenant_id=$1 AND window_started_at<$3)`, policy.TenantID, now.Add(-policy.PublishedOutboxAfter), now.Add(-policy.RateWindowAfter)).Scan(&result.PublishedOutbox, &result.ExpiredRates)
	}
	if err != nil {
		return retention.Result{}, fmt.Errorf("execute retention batch: %w", err)
	}
	result.RunID, err = newUUID()
	if err != nil {
		return retention.Result{}, err
	}
	result.CompletedAt = r.clock().UTC()
	if _, err = tx.ExecContext(ctx, `INSERT INTO retention_runs (id,tenant_id,mode,published_outbox_count,retained_idempotency_count,expired_rate_window_count,correlation_id,application_version,started_at,completed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, result.RunID, policy.TenantID, mode, result.PublishedOutbox, result.RetainedIdempotency, result.ExpiredRates, correlationID, buildVersion(), result.StartedAt, result.CompletedAt); err != nil {
		return retention.Result{}, err
	}
	metadata, _ := json.Marshal(map[string]any{"mode": mode, "published_outbox_count": result.PublishedOutbox, "retained_idempotency_count": result.RetainedIdempotency, "expired_rate_window_count": result.ExpiredRates})
	if err = appendControlledAuditPayload(ctx, tx, result.RunID, AuditEvent{
		TenantID: policy.TenantID, EventType: "retention.completed", TargetType: "retention_run",
		TargetID: result.RunID, Outcome: "succeeded", CorrelationID: correlationID,
		OccurredAt: result.CompletedAt,
	}, metadata); err != nil {
		return retention.Result{}, err
	}
	if err = tx.Commit(); err != nil {
		return retention.Result{}, err
	}
	return result, nil
}

func deleteBatch(ctx context.Context, tx *sql.Tx, statement string, arguments ...any) (int64, error) {
	result, err := tx.ExecContext(ctx, statement, arguments...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
