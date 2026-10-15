package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/webhookdelivery"
)

// WebhookDeliveryJobRepository owns only mutable dispatch coordination. It
// appends delivery_attempts inside the same transaction that advances a job,
// leaving the transfer and original outbox payload untouched.
type WebhookDeliveryJobRepository struct {
	database *sql.DB
	clock    func() time.Time
}

func NewWebhookDeliveryJobRepository(database *sql.DB, clock func() time.Time) (*WebhookDeliveryJobRepository, error) {
	if database == nil {
		return nil, errors.New("webhook delivery job database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &WebhookDeliveryJobRepository{database: database, clock: clock}, nil
}

func (r *WebhookDeliveryJobRepository) Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]webhookdelivery.Job, error) {
	if workerID == "" || limit < 1 || limit > 100 || lease <= 0 || lease > 10*time.Minute {
		return nil, errors.New("valid webhook claim arguments are required")
	}
	rows, err := r.database.QueryContext(ctx, `SELECT * FROM public.controlled_claim_webhook_delivery_v1($1,$2,$3)`, lease.Seconds(), limit, workerID)
	if err != nil {
		return nil, fmt.Errorf("claim webhook delivery jobs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	jobs := make([]webhookdelivery.Job, 0, limit)
	for rows.Next() {
		var job webhookdelivery.Job
		if err := rows.Scan(&job.ID, &job.TenantID, &job.TransferID, &job.OutboxEventID, &job.WebhookID, &job.EventID, &job.EventType, &job.EndpointURL, &job.SigningKeyReference, &job.SigningKeyID, &job.Payload, &job.AttemptNumber, new(time.Time), &job.ClaimGeneration); err != nil {
			return nil, fmt.Errorf("scan webhook delivery job: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webhook delivery jobs: %w", err)
	}
	// Release the claim result before using a scoped item transaction. A worker
	// with a one-connection pool must not wait on its own open result set.
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range jobs {
		job := &jobs[i]
		tx, err := beginClaimTransaction(ctx, r.database, "webhook_delivery", job.ID, workerID, job.ClaimGeneration, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return nil, err
		}
		var parent sql.NullString
		err = tx.QueryRowContext(ctx, `SELECT diagnostic_traceparent FROM outbox_events WHERE id=$1 AND tenant_id=$2`, job.OutboxEventID, job.TenantID).Scan(&parent)
		_ = tx.Rollback()
		if err != nil {
			return nil, err
		}
		job.DiagnosticTraceParent = parent.String
	}
	return jobs, nil
}

func (r *WebhookDeliveryJobRepository) Complete(ctx context.Context, completion webhookdelivery.Completion) error {
	if completion.ClaimGeneration < 1 {
		return ErrStaleClaim
	}
	if completion.JobID == "" || completion.WorkerID == "" || completion.AttemptNumber < 1 || completion.CompletedAt.IsZero() || (completion.Status != webhookdelivery.StatusDelivered && completion.Status != webhookdelivery.StatusRetrying && completion.Status != webhookdelivery.StatusDead) || completion.Status == webhookdelivery.StatusRetrying && completion.RetryAt == nil {
		return errors.New("complete webhook delivery completion is required")
	}
	tx, err := beginClaimTransaction(ctx, r.database, "webhook_delivery", completion.JobID, completion.WorkerID, completion.ClaimGeneration, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var retryAt any
	if completion.RetryAt != nil {
		retryAt = completion.RetryAt.UTC()
	}
	var completed bool
	if err := tx.QueryRowContext(ctx, `SELECT public.controlled_complete_delivery_v1($1,$2,$3,$4,$5,$6,$7,$8,$9)`, completion.JobID, completion.WorkerID, completion.ClaimGeneration, completion.AttemptNumber, completion.Status, completion.CompletedAt, retryAt, completion.ResponseClass, completion.ErrorCode).Scan(&completed); err != nil {
		return err
	}
	if !completed {
		return ErrStaleClaim
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

var _ webhookdelivery.Store = (*WebhookDeliveryJobRepository)(nil)

func (r *WebhookDeliveryJobRepository) RenewClaim(ctx context.Context, owner, id string, generation int64, duration time.Duration) error {
	return renewWorkerClaim(ctx, r.database, "webhook_delivery", owner, id, generation, duration)
}
