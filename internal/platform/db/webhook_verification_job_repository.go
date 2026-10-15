package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/webhookverification"
)

// WebhookVerificationJobRepository owns only mutable verification work. The
// endpoint activation itself remains conditional on an authenticated proof and
// creates append-only developer webhook evidence in the same transaction.
type WebhookVerificationJobRepository struct {
	database *sql.DB
	clock    func() time.Time
}

func NewWebhookVerificationJobRepository(database *sql.DB, clock func() time.Time) (*WebhookVerificationJobRepository, error) {
	if database == nil {
		return nil, errors.New("webhook verification database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &WebhookVerificationJobRepository{database: database, clock: clock}, nil
}

func (r *WebhookVerificationJobRepository) Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]webhookverification.Job, error) {
	if strings.TrimSpace(workerID) == "" || limit < 1 || limit > 100 || lease <= 0 || lease > 10*time.Minute {
		return nil, errors.New("bounded webhook verification claim is required")
	}
	rows, err := r.database.QueryContext(ctx, `SELECT * FROM public.controlled_claim_webhook_verification_v1($1,$2,$3)`, lease.Seconds(), limit, workerID)
	if err != nil {
		return nil, fmt.Errorf("claim webhook verification jobs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	jobs := make([]webhookverification.Job, 0, limit)
	for rows.Next() {
		var job webhookverification.Job
		if err := rows.Scan(&job.ID, &job.TenantID, &job.WebhookID, &job.EndpointURL, &job.SigningKeyReference, &job.SigningKeyID, &job.Challenge, &job.ExpiresAt, &job.AttemptNumber, &job.ClaimGeneration); err != nil {
			return nil, fmt.Errorf("scan webhook verification job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *WebhookVerificationJobRepository) Complete(ctx context.Context, completion webhookverification.Completion) error {
	if completion.ClaimGeneration < 1 {
		return ErrStaleClaim
	}
	if strings.TrimSpace(completion.JobID) == "" || strings.TrimSpace(completion.WorkerID) == "" || completion.CompletedAt.IsZero() || !validVerificationStatus(completion.Status) {
		return errors.New("complete webhook verification job is required")
	}
	tx, err := beginClaimTransaction(ctx, r.database, "webhook_verification", completion.JobID, completion.WorkerID, completion.ClaimGeneration, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var retryAt any
	if completion.RetryAt != nil {
		retryAt = completion.RetryAt.UTC()
	}
	var completed bool
	if err := tx.QueryRowContext(ctx, `SELECT public.controlled_complete_webhook_verification_v1($1,$2,$3,$4,$5,$6,$7,$8)`, completion.JobID, completion.WorkerID, completion.ClaimGeneration, completion.Status, completion.CompletedAt.UTC(), retryAt, completion.ErrorCode, completion.ResponseClass).Scan(&completed); err != nil {
		return err
	}
	if !completed {
		return ErrStaleClaim
	}
	return tx.Commit()
}

func validVerificationStatus(status webhookverification.Status) bool {
	return status == webhookverification.StatusRetrying || status == webhookverification.StatusVerified || status == webhookverification.StatusDead
}

var _ webhookverification.Store = (*WebhookVerificationJobRepository)(nil)

func (r *WebhookVerificationJobRepository) RenewClaim(ctx context.Context, owner, id string, generation int64, duration time.Duration) error {
	return renewWorkerClaim(ctx, r.database, "webhook_verification", owner, id, generation, duration)
}
