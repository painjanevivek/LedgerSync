// Package webhookverification proves endpoint control through durable,
// signed, server-initiated challenges. It never accepts a challenge echoed by
// an API client.
package webhookverification

import (
	"context"
	"errors"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/lease"
	"strings"
	"time"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusRetrying Status = "retrying"
	StatusVerified Status = "verified"
	StatusDead     Status = "dead"
)

type Job struct {
	ID, TenantID, WebhookID, EndpointURL, SigningKeyReference, SigningKeyID string
	Challenge                                                               []byte
	ExpiresAt                                                               time.Time
	AttemptNumber                                                           int
	ClaimGeneration                                                         int64
}

type Outcome struct {
	ResponseClass string
	Retryable     bool
}

type Completion struct {
	JobID, WorkerID, ResponseClass, ErrorCode string
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
	Verify(context.Context, Job) (Outcome, error)
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

func NewWorker(store Store, sender Sender, clock func() time.Time, config Config) (*Worker, error) {
	if store == nil || sender == nil || strings.TrimSpace(config.WorkerID) == "" {
		return nil, errors.New("webhook verification store, sender, and worker ID are required")
	}
	if clock == nil {
		clock = time.Now
	}
	if config.BatchSize < 1 {
		config.BatchSize = 5
	}
	if config.BatchSize > 100 {
		return nil, errors.New("verification batch exceeds 100 jobs")
	}
	if config.ClaimLease <= 0 {
		config.ClaimLease = 30 * time.Second
	}
	if config.MaxAttempts < 1 {
		config.MaxAttempts = 8
	}
	return &Worker{store: store, sender: sender, clock: clock, workerID: config.WorkerID, batchSize: config.BatchSize, claimLease: config.ClaimLease, maxAttempts: config.MaxAttempts, onItem: config.OnItem}, nil
}

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
		completion := Completion{JobID: job.ID, WorkerID: w.workerID, CompletedAt: now, ClaimGeneration: job.ClaimGeneration}
		if !job.ExpiresAt.After(now) {
			completion.Status, completion.ResponseClass, completion.ErrorCode = StatusDead, "challenge_expired", "challenge_expired"
		} else {
			var outcome Outcome
			verifyErr := lease.Run(ctx, w.claimLease, func(c context.Context) error {
				return w.store.RenewClaim(c, w.workerID, job.ID, job.ClaimGeneration, w.claimLease)
			}, func(c context.Context) error {
				var err error
				outcome, err = w.sender.Verify(c, job)
				return err
			})
			if errors.Is(verifyErr, lease.ErrLost) {
				return 0, verifyErr
			}
			now = w.clock().UTC()
			completion.CompletedAt = now
			completion.ResponseClass = outcome.ResponseClass
			if completion.ResponseClass == "" {
				completion.ResponseClass = "verification_error"
			}
			switch {
			case verifyErr == nil:
				completion.Status = StatusVerified
			case outcome.Retryable && job.AttemptNumber < w.maxAttempts:
				retryAt := now.Add(retryDelay(job.AttemptNumber))
				completion.Status, completion.ErrorCode, completion.RetryAt = StatusRetrying, completion.ResponseClass, &retryAt
			default:
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
