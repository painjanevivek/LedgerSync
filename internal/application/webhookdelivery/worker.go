package webhookdelivery

import (
	"context"
	"errors"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/lease"
	"strings"
	"time"
)

// Status is the persisted state of one webhook delivery attempt or job.
// Completed attempts are append-only evidence; jobs alone are leased and
// changed as they progress.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRetrying  Status = "retrying"
	StatusDelivered Status = "delivered"
	StatusDead      Status = "dead"
)

// Job is a claimed immutable event plus the current endpoint material. An
// empty endpoint means the endpoint was disabled after scheduling and must be
// recorded as a dead delivery without an outbound request.
type Job struct {
	ID, TenantID, TransferID, OutboxEventID, WebhookID string
	EventID, EventType                                 string
	EndpointURL, SigningKeyReference, SigningKeyID     string
	Payload                                            []byte
	AttemptNumber                                      int
	ClaimGeneration                                    int64
	DiagnosticTraceParent                              string
}

// Completion atomically advances the mutable job and appends its immutable
// attempt evidence in the store implementation.
type Completion struct {
	JobID, WorkerID, ResponseClass, ErrorCode string
	AttemptNumber                             int
	ClaimGeneration                           int64
	Status                                    Status
	CompletedAt                               time.Time
	RetryAt                                   *time.Time
}

type Store interface {
	Claim(context.Context, string, int, time.Duration) ([]Job, error)
	Complete(context.Context, Completion) error
	RenewClaim(context.Context, string, string, int64, time.Duration) error
}

type Sender interface {
	Dispatch(context.Context, Delivery) (Outcome, error)
}

type Config struct {
	WorkerID    string
	BatchSize   int
	ClaimLease  time.Duration
	MaxAttempts int
	OnItem      func(string)
}

type Worker struct {
	store       Store
	sender      Sender
	clock       func() time.Time
	workerID    string
	batchSize   int
	claimLease  time.Duration
	maxAttempts int
	onItem      func(string)
}

func NewWorker(store Store, sender Sender, clock func() time.Time, cfg Config) (*Worker, error) {
	if store == nil || sender == nil || strings.TrimSpace(cfg.WorkerID) == "" {
		return nil, errors.New("webhook store, sender, and worker ID are required")
	}
	if clock == nil {
		clock = time.Now
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 5
	}
	if cfg.BatchSize > 100 {
		return nil, errors.New("webhook batch exceeds 100 jobs")
	}
	if cfg.ClaimLease <= 0 {
		cfg.ClaimLease = 30 * time.Second
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 8
	}
	return &Worker{store: store, sender: sender, clock: clock, workerID: cfg.WorkerID, batchSize: cfg.BatchSize, claimLease: cfg.ClaimLease, maxAttempts: cfg.MaxAttempts, onItem: cfg.OnItem}, nil
}

// RunOnce processes every currently leased job. A returned error means durable
// evidence could not be written; a rejected endpoint is evidence, not a worker
// failure. Receivers must deduplicate by X-LedgerSync-Event-ID because a crash
// after HTTP acceptance but before Complete is intentionally at-least-once.
func (w *Worker) RunOnce(ctx context.Context) (int, error) {
	jobs, err := w.store.Claim(ctx, w.workerID, w.batchSize, w.claimLease)
	if err != nil {
		return 0, err
	}
	for _, job := range jobs {
		if w.onItem != nil {
			w.onItem(job.ID)
		}
		now := w.clock().UTC()
		completion := Completion{JobID: job.ID, WorkerID: w.workerID, AttemptNumber: job.AttemptNumber, CompletedAt: now, ClaimGeneration: job.ClaimGeneration}
		if strings.TrimSpace(job.EndpointURL) == "" || strings.TrimSpace(job.SigningKeyReference) == "" || strings.TrimSpace(job.SigningKeyID) == "" {
			completion.Status, completion.ResponseClass, completion.ErrorCode = StatusDead, "endpoint_inactive", "endpoint_inactive"
		} else {
			var outcome Outcome
			sendErr := lease.Run(ctx, w.claimLease, func(c context.Context) error {
				return w.store.RenewClaim(c, w.workerID, job.ID, job.ClaimGeneration, w.claimLease)
			}, func(c context.Context) error {
				var err error
				outcome, err = w.sender.Dispatch(c, Delivery{ID: job.ID, EventID: job.EventID, EventType: job.EventType, EndpointURL: job.EndpointURL, SigningKeyReference: job.SigningKeyReference, SigningKeyID: job.SigningKeyID, Payload: job.Payload, DiagnosticTraceParent: job.DiagnosticTraceParent})
				return err
			})
			if errors.Is(sendErr, lease.ErrLost) {
				return 0, sendErr
			}
			now = w.clock().UTC()
			completion.CompletedAt = now
			completion.ResponseClass = outcome.ResponseClass
			if completion.ResponseClass == "" {
				completion.ResponseClass = "dispatch_error"
			}
			if sendErr == nil {
				completion.Status = StatusDelivered
			} else if outcome.Retryable && job.AttemptNumber < w.maxAttempts {
				retryAt := now.Add(retryDelay(job.AttemptNumber))
				completion.Status, completion.ErrorCode, completion.RetryAt = StatusRetrying, completion.ResponseClass, &retryAt
			} else {
				completion.Status, completion.ErrorCode = StatusDead, completion.ResponseClass
			}
		}
		if err := w.store.Complete(ctx, completion); err != nil {
			return 0, err
		}
	}
	return len(jobs), nil
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Second << min(attempt-1, 9)
	if delay > 10*time.Minute {
		return 10 * time.Minute
	}
	return delay
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
