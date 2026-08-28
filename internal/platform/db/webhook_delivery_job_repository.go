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
	if workerID == "" || limit < 1 || lease <= 0 {
		return nil, errors.New("valid webhook claim arguments are required")
	}
	now := r.clock().UTC()
	rows, err := r.database.QueryContext(ctx, `
WITH candidates AS (
  SELECT id
  FROM webhook_delivery_jobs
  WHERE status IN ('pending','retrying')
    AND available_at <= $1
    AND (claimed_until IS NULL OR claimed_until <= $1)
    AND EXISTS (SELECT 1 FROM outbox_events event WHERE event.id=webhook_delivery_jobs.outbox_event_id AND event.published_at IS NOT NULL AND event.dead_at IS NULL)
  ORDER BY available_at,created_at,id
  LIMIT $2
  FOR UPDATE SKIP LOCKED
)
UPDATE webhook_delivery_jobs AS job
SET claim_owner=$3,claimed_until=$4,started_at=$1,updated_at=$1
FROM candidates
WHERE job.id=candidates.id
RETURNING job.id::text,job.tenant_id::text,job.transfer_id::text,job.outbox_event_id::text,job.webhook_id::text,
  job.event_id::text,job.event_type,
  COALESCE((SELECT endpoint.endpoint_url FROM developer_webhook_endpoints endpoint WHERE endpoint.id=job.webhook_id AND endpoint.tenant_id=job.tenant_id AND endpoint.status='active'),''),
  COALESCE((SELECT endpoint.signing_key_reference FROM developer_webhook_endpoints endpoint WHERE endpoint.id=job.webhook_id AND endpoint.tenant_id=job.tenant_id AND endpoint.status='active'),''),
  COALESCE((SELECT endpoint.signing_key_id FROM developer_webhook_endpoints endpoint WHERE endpoint.id=job.webhook_id AND endpoint.tenant_id=job.tenant_id AND endpoint.status='active'),''),
  job.payload::text,job.attempt_number,job.started_at`, now, limit, workerID, now.Add(lease))
	if err != nil {
		return nil, fmt.Errorf("claim webhook delivery jobs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	jobs := make([]webhookdelivery.Job, 0, limit)
	for rows.Next() {
		var job webhookdelivery.Job
		if err := rows.Scan(&job.ID, &job.TenantID, &job.TransferID, &job.OutboxEventID, &job.WebhookID, &job.EventID, &job.EventType, &job.EndpointURL, &job.SigningKeyReference, &job.SigningKeyID, &job.Payload, &job.AttemptNumber, new(time.Time)); err != nil {
			return nil, fmt.Errorf("scan webhook delivery job: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webhook delivery jobs: %w", err)
	}
	return jobs, nil
}

func (r *WebhookDeliveryJobRepository) Complete(ctx context.Context, completion webhookdelivery.Completion) error {
	if completion.JobID == "" || completion.WorkerID == "" || completion.AttemptNumber < 1 || completion.CompletedAt.IsZero() || (completion.Status != webhookdelivery.StatusDelivered && completion.Status != webhookdelivery.StatusRetrying && completion.Status != webhookdelivery.StatusDead) || completion.Status == webhookdelivery.StatusRetrying && completion.RetryAt == nil {
		return errors.New("complete webhook delivery completion is required")
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var tenantID, transferID, outboxEventID, webhookID string
	var startedAt time.Time
	err = tx.QueryRowContext(ctx, `SELECT tenant_id::text,transfer_id::text,outbox_event_id::text,webhook_id::text,started_at FROM webhook_delivery_jobs WHERE id=$1 AND claim_owner=$2 FOR UPDATE`, completion.JobID, completion.WorkerID).Scan(&tenantID, &transferID, &outboxEventID, &webhookID, &startedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("webhook delivery job is not claimed by this worker")
	}
	if err != nil {
		return fmt.Errorf("lock webhook delivery job: %w", err)
	}
	attemptID, err := newUUID()
	if err != nil {
		return err
	}
	dueAt := completion.CompletedAt
	if completion.RetryAt != nil {
		dueAt = *completion.RetryAt
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO delivery_attempts(id,tenant_id,transfer_id,outbox_event_id,delivery_kind,endpoint_reference,attempt_number,status,response_class,sanitized_error_code,due_at,started_at,completed_at)VALUES($1,$2,$3,$4,'webhook',$5,$6,$7,NULLIF($8,''),NULLIF($9,''),$10,$11,$12)`, attemptID, tenantID, transferID, outboxEventID, webhookID, completion.AttemptNumber, completion.Status, completion.ResponseClass, completion.ErrorCode, dueAt, startedAt, completion.CompletedAt); err != nil {
		return fmt.Errorf("append webhook delivery attempt: %w", err)
	}
	var retryAt any
	if completion.RetryAt != nil {
		retryAt = *completion.RetryAt
	}
	statement := `UPDATE webhook_delivery_jobs SET status=$3,attempt_number=CASE WHEN $3='retrying' THEN attempt_number+1 ELSE attempt_number END,available_at=COALESCE($4,available_at),claim_owner=NULL,claimed_until=NULL,last_error_code=NULLIF($5,''),completed_at=CASE WHEN $3 IN ('delivered','dead') THEN $6 ELSE NULL END,updated_at=$6 WHERE id=$1 AND claim_owner=$2`
	result, err := tx.ExecContext(ctx, statement, completion.JobID, completion.WorkerID, completion.Status, retryAt, completion.ErrorCode, completion.CompletedAt)
	if err != nil {
		return fmt.Errorf("advance webhook delivery job: %w", err)
	}
	if err = requireOneRow(result, "advance webhook delivery job"); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

var _ webhookdelivery.Store = (*WebhookDeliveryJobRepository)(nil)
