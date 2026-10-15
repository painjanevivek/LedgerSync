package db

import (
	"context"
	"database/sql"
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
	tx, err := beginTenantTransaction(ctx, r.database, TenantScope{TenantID: policy.TenantID, ActorID: "system:retention"}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return retention.Result{}, err
	}
	defer func() { _ = tx.Rollback() }()
	result := retention.Result{CorrelationID: correlationID}
	if err := tx.QueryRowContext(ctx, `SELECT run_id,mode,published_outbox,retained_idempotency,expired_rates,started_at,completed_at FROM public.controlled_run_retention_v1($1,$2,$3,$4,$5,$6,$7,$8)`, policy.TenantID, apply, policy.BatchSize, policy.PublishedOutboxAfter.Seconds(), policy.RateWindowAfter.Seconds(), correlationID, buildVersion(), r.clock().UTC()).Scan(&result.RunID, &result.Mode, &result.PublishedOutbox, &result.RetainedIdempotency, &result.ExpiredRates, &result.StartedAt, &result.CompletedAt); err != nil {
		return retention.Result{}, fmt.Errorf("execute controlled retention: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return retention.Result{}, err
	}
	return result, nil
}
