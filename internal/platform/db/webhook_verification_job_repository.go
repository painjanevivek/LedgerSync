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
	if strings.TrimSpace(workerID) == "" || limit < 1 || limit > 100 || lease <= 0 {
		return nil, errors.New("bounded webhook verification claim is required")
	}
	now := r.clock().UTC()
	claimedUntil := now.Add(lease)
	rows, err := r.database.QueryContext(ctx, `WITH candidates AS (
  SELECT id FROM webhook_endpoint_verification_jobs
  WHERE status IN ('pending','retrying') AND available_at <= $1 AND (claimed_until IS NULL OR claimed_until <= $1)
  ORDER BY available_at,created_at,id LIMIT $2 FOR UPDATE SKIP LOCKED
), claimed AS (
  UPDATE webhook_endpoint_verification_jobs job SET claim_owner=$3,claimed_until=$4,updated_at=$1
  FROM candidates WHERE job.id=candidates.id
  RETURNING job.id,job.tenant_id,job.webhook_id,job.challenge,job.expires_at,job.attempt_number
)
SELECT claimed.id::text,claimed.tenant_id::text,claimed.webhook_id::text,endpoint.endpoint_url,endpoint.signing_key_reference,endpoint.signing_key_id,claimed.challenge,claimed.expires_at,claimed.attempt_number
FROM claimed JOIN developer_webhook_endpoints endpoint ON endpoint.id=claimed.webhook_id AND endpoint.tenant_id=claimed.tenant_id
WHERE endpoint.status='pending_verification'`, now, limit, workerID, claimedUntil)
	if err != nil {
		return nil, fmt.Errorf("claim webhook verification jobs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	jobs := make([]webhookverification.Job, 0, limit)
	for rows.Next() {
		var job webhookverification.Job
		if err := rows.Scan(&job.ID, &job.TenantID, &job.WebhookID, &job.EndpointURL, &job.SigningKeyReference, &job.SigningKeyID, &job.Challenge, &job.ExpiresAt, &job.AttemptNumber); err != nil {
			return nil, fmt.Errorf("scan webhook verification job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *WebhookVerificationJobRepository) Complete(ctx context.Context, completion webhookverification.Completion) error {
	if strings.TrimSpace(completion.JobID) == "" || strings.TrimSpace(completion.WorkerID) == "" || completion.CompletedAt.IsZero() || !validVerificationStatus(completion.Status) {
		return errors.New("complete webhook verification job is required")
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var tenantID, webhookID, actorID, correlationID string
	var attempt, version int
	var expiresAt time.Time
	var status string
	err = tx.QueryRowContext(ctx, `SELECT job.tenant_id::text,job.webhook_id::text,job.actor_subject_id,job.correlation_id::text,job.attempt_number,job.expires_at,job.status,endpoint.version
FROM webhook_endpoint_verification_jobs job JOIN developer_webhook_endpoints endpoint ON endpoint.id=job.webhook_id AND endpoint.tenant_id=job.tenant_id
WHERE job.id=$1 AND job.claim_owner=$2 FOR UPDATE OF job,endpoint`, completion.JobID, completion.WorkerID).Scan(&tenantID, &webhookID, &actorID, &correlationID, &attempt, &expiresAt, &status, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("webhook verification job lease is unavailable")
	}
	if err != nil {
		return err
	}
	if status != "pending" && status != "retrying" {
		return errors.New("webhook verification job is no longer actionable")
	}
	if completion.Status == webhookverification.StatusVerified && !expiresAt.After(completion.CompletedAt.UTC()) {
		return errors.New("expired challenge cannot verify a webhook")
	}
	var retryAt any
	if completion.RetryAt != nil {
		retryAt = completion.RetryAt.UTC()
	}
	if completion.Status == webhookverification.StatusVerified {
		result, updateErr := tx.ExecContext(ctx, `UPDATE developer_webhook_endpoints SET status='active',version=version+1,challenge_digest=NULL,challenge_expires_at=NULL,verified_at=$3,updated_at=$3 WHERE tenant_id=$1 AND id=$2 AND status='pending_verification'`, tenantID, webhookID, completion.CompletedAt.UTC())
		if updateErr != nil {
			return updateErr
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return errors.New("webhook endpoint could not be activated")
		}
		if err := insertWebhookEvent(ctx, tx, tenantID, webhookID, "verified", int64(version+1), actorID, correlationID, completion.CompletedAt.UTC(), map[string]any{"verification_job_id": completion.JobID, "response_class": completion.ResponseClass}); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE webhook_endpoint_verification_jobs SET status=$3,attempt_number=CASE WHEN $3='retrying' THEN attempt_number+1 ELSE attempt_number END,available_at=COALESCE($4,available_at),claim_owner=NULL,claimed_until=NULL,last_error_code=NULLIF($5,''),completed_at=CASE WHEN $3 IN ('verified','dead') THEN $6 ELSE NULL END,updated_at=$6 WHERE id=$1 AND claim_owner=$2`, completion.JobID, completion.WorkerID, completion.Status, retryAt, completion.ErrorCode, completion.CompletedAt.UTC())
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return errors.New("webhook verification completion was not persisted")
	}
	return tx.Commit()
}

func validVerificationStatus(status webhookverification.Status) bool {
	return status == webhookverification.StatusRetrying || status == webhookverification.StatusVerified || status == webhookverification.StatusDead
}

var _ webhookverification.Store = (*WebhookVerificationJobRepository)(nil)
