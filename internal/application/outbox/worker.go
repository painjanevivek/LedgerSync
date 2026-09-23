// Package outbox delivers committed domain events after the financial
// transaction has succeeded. It intentionally treats delivery as at-least-once:
// consumers must deduplicate by event ID and apply balance versions monotonically.
package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrInvalidEvent = errors.New("invalid outbox event")

type Event struct {
	ID               string
	TenantID         string
	TransferID       string
	AccountID        string
	AggregateType    string
	AggregateID      string
	EventType        string
	AggregateVersion int64
	Payload          []byte
	OccurredAt       time.Time
	AttemptCount     int
}

// Store owns the durable delivery state. A claim lease makes concurrent
// workers safe while allowing work to be recovered after a process failure.
type Store interface {
	Claim(context.Context, string, int, time.Duration) ([]Event, error)
	MarkPublished(context.Context, string, string, time.Time) error
	Reschedule(context.Context, string, string, time.Time, string) error
	MarkDead(context.Context, string, string, time.Time, string) error
}

// Publisher must publish the event ID and aggregate version unchanged. It may
// receive an event again if a process dies after publish and before its durable
// acknowledgement; that is the deliberate at-least-once boundary.
type Publisher interface {
	Publish(context.Context, Event) error
}

type Metrics interface {
	ObservePublished(Event)
	ObserveRetry(Event, error)
	ObserveDead(Event, error)
}

type Worker struct {
	store       Store
	publisher   Publisher
	metrics     Metrics
	clock       func() time.Time
	workerID    string
	batchSize   int
	claimLease  time.Duration
	maxAttempts int
	onItem      func(string)
}

type Config struct {
	WorkerID    string
	BatchSize   int
	ClaimLease  time.Duration
	MaxAttempts int
	OnItem      func(string)
}

func NewWorker(store Store, publisher Publisher, metrics Metrics, clock func() time.Time, cfg Config) (*Worker, error) {
	if store == nil || publisher == nil {
		return nil, errors.New("outbox store and publisher are required")
	}
	if clock == nil {
		clock = time.Now
	}
	if cfg.WorkerID == "" {
		return nil, errors.New("outbox worker ID is required")
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	if cfg.ClaimLease <= 0 {
		cfg.ClaimLease = 30 * time.Second
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 12
	}
	return &Worker{store: store, publisher: publisher, metrics: metrics, clock: clock, workerID: cfg.WorkerID, batchSize: cfg.BatchSize, claimLease: cfg.ClaimLease, maxAttempts: cfg.MaxAttempts, onItem: cfg.OnItem}, nil
}

// RunOnce returns the number of claimed events. An individual event failure is
// persisted as retry/dead state and does not prevent later events in the batch.
func (w *Worker) RunOnce(ctx context.Context) (int, error) {
	events, err := w.store.Claim(ctx, w.workerID, w.batchSize, w.claimLease)
	if err != nil {
		return 0, fmt.Errorf("claim outbox events: %w", err)
	}
	for _, event := range events {
		if w.onItem != nil {
			w.onItem(event.ID)
		}
		if err := validate(event); err != nil {
			if persistErr := w.store.MarkDead(ctx, w.workerID, event.ID, w.clock().UTC(), "invalid_event"); persistErr != nil {
				return len(events), fmt.Errorf("mark invalid event dead: %w", persistErr)
			}
			w.observeDead(event, err)
			continue
		}
		if err := w.publisher.Publish(ctx, event); err != nil {
			if persistErr := w.handlePublishFailure(ctx, event, err); persistErr != nil {
				return len(events), persistErr
			}
			continue
		}
		if err := w.store.MarkPublished(ctx, w.workerID, event.ID, w.clock().UTC()); err != nil {
			// Do not publish a compensating message here. A future claim may
			// deliver the same immutable event; consumers handle that safely.
			return len(events), fmt.Errorf("acknowledge published event: %w", err)
		}
		if w.metrics != nil {
			w.metrics.ObservePublished(event)
		}
	}
	return len(events), nil
}

func (w *Worker) handlePublishFailure(ctx context.Context, event Event, publishErr error) error {
	now := w.clock().UTC()
	if event.AttemptCount >= w.maxAttempts {
		if err := w.store.MarkDead(ctx, w.workerID, event.ID, now, "publish_failed"); err != nil {
			return fmt.Errorf("persist dead outbox event: %w", err)
		}
		w.observeDead(event, publishErr)
		return nil
	}
	next := now.Add(backoff(event.AttemptCount))
	if err := w.store.Reschedule(ctx, w.workerID, event.ID, next, "publish_failed"); err != nil {
		return fmt.Errorf("persist outbox retry: %w", err)
	}
	if w.metrics != nil {
		w.metrics.ObserveRetry(event, publishErr)
	}
	return nil
}

func validate(event Event) error {
	aggregateType, aggregateID := event.AggregateType, event.AggregateID
	if aggregateType == "" && event.AccountID != "" {
		aggregateType = "account_balance"
	}
	if aggregateID == "" {
		aggregateID = event.AccountID
	}
	if event.ID == "" || event.TenantID == "" || aggregateType == "" || aggregateID == "" || event.EventType == "" || event.AggregateVersion < 0 || len(event.Payload) == 0 {
		return ErrInvalidEvent
	}
	return nil
}

func backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 10 {
		attempt = 10
	}
	return time.Second * time.Duration(1<<(attempt-1))
}

func (w *Worker) observeDead(event Event, err error) {
	if w.metrics != nil {
		w.metrics.ObserveDead(event, err)
	}
}
