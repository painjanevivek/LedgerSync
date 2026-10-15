package db

import (
	"context"
	"database/sql"
	"errors"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
	"time"
)

type SchedulerRepository struct{ database *sql.DB }

func NewSchedulerRepository(database *sql.DB) (*SchedulerRepository, error) {
	if database == nil {
		return nil, errors.New("scheduler database is required")
	}
	return &SchedulerRepository{database}, nil
}
func (r *SchedulerRepository) Acquire(ctx context.Context, tenant, owner string, duration time.Duration) (int64, error) {
	var token sql.NullInt64
	err := r.database.QueryRowContext(ctx, `SELECT public.acquire_reconciliation_scheduler($1,$2,$3)`, tenant, owner, duration.Milliseconds()).Scan(&token)
	return token.Int64, err
}
func (r *SchedulerRepository) Renew(ctx context.Context, tenant, owner string, token int64, duration time.Duration) error {
	var renewed bool
	err := r.database.QueryRowContext(ctx, `SELECT public.renew_reconciliation_scheduler($1,$2,$3,$4)`, tenant, owner, token, duration.Milliseconds()).Scan(&renewed)
	if err != nil {
		return err
	}
	if !renewed {
		return ErrStaleClaim
	}
	return nil
}
func (r *SchedulerRepository) Claim(ctx context.Context, tenant, owner string, token int64, interval time.Duration) (string, error) {
	var job sql.NullString
	err := r.database.QueryRowContext(ctx, `SELECT public.claim_reconciliation_schedule($1,$2,$3,$4)`, tenant, owner, token, int64(interval/time.Second)).Scan(&job)
	return job.String, err
}
func (r *SchedulerRepository) Complete(ctx context.Context, tenant, owner string, token int64, job string, result reconciliation.Result) error {
	_, err := r.database.ExecContext(ctx, `SELECT public.complete_reconciliation_schedule($1,$2,$3,$4,$5,$6)`, tenant, owner, token, job, result.ID, string(result.Status))
	return err
}
func (r *SchedulerRepository) Release(ctx context.Context, tenant, owner string, token int64) error {
	_, err := r.database.ExecContext(ctx, `SELECT public.release_reconciliation_scheduler($1,$2,$3)`, tenant, owner, token)
	return err
}
