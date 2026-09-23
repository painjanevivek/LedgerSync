package observability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

const (
	WorkerQueueOutbox              = "outbox_publish"
	WorkerQueueWebhookDelivery     = "webhook_delivery"
	WorkerQueueWebhookVerification = "webhook_verification"
	WorkerQueueProjection          = "balance_projection"
	workerQueueCount               = 4
)

var workerQueues = [workerQueueCount]string{
	WorkerQueueOutbox,
	WorkerQueueWebhookDelivery,
	WorkerQueueWebhookVerification,
	WorkerQueueProjection,
}

// WorkerProgressReport contains bounded operational state. The item hash is
// deliberately not suitable for a metric label; it is available only for a
// controlled probe or diagnostic log that needs to correlate the last item.
type WorkerProgressReport struct {
	Queue           string
	HeartbeatAt     time.Time
	LastStartedAt   time.Time
	LastCompletedAt time.Time
	FailureAge      *time.Duration
	ProgressAge     time.Duration
	LastItemHash    string
	Active          bool
}

type WorkerProgressObserver interface {
	ObserveWorkerProgress(context.Context, WorkerProgressReport)
}

type workerProgressState struct {
	active          bool
	lastStartedAt   time.Time
	lastCompletedAt time.Time
	lastFailureAt   time.Time
	lastItemHash    string
}

// WorkerProgressMonitor emits from its own goroutine, so a blocked queue loop
// still produces a fresh process heartbeat and an increasing active age.
type WorkerProgressMonitor struct {
	observer WorkerProgressObserver
	interval time.Duration
	clock    func() time.Time

	mu     sync.RWMutex
	queues map[string]workerProgressState
}

func NewWorkerProgressMonitor(observer WorkerProgressObserver, interval time.Duration, clock func() time.Time) (*WorkerProgressMonitor, error) {
	if observer == nil || interval <= 0 || interval > time.Minute {
		return nil, errors.New("worker progress monitor requires an observer and a 1ns..1m interval")
	}
	if clock == nil {
		clock = time.Now
	}
	now := clock().UTC()
	queues := make(map[string]workerProgressState, workerQueueCount)
	for _, queue := range workerQueues {
		queues[queue] = workerProgressState{lastCompletedAt: now}
	}
	return &WorkerProgressMonitor{observer: observer, interval: interval, clock: clock, queues: queues}, nil
}

func (monitor *WorkerProgressMonitor) MarkStarted(queue, itemID string) error {
	return monitor.MarkItem(queue, itemID)
}

func (monitor *WorkerProgressMonitor) MarkItem(queue, itemID string) error {
	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	state, ok := monitor.queues[queue]
	if !ok {
		return errors.New("worker progress queue is not registered")
	}
	state.active = true
	state.lastStartedAt = monitor.clock().UTC()
	state.lastItemHash = hashWorkerItem(itemID)
	monitor.queues[queue] = state
	return nil
}

func (monitor *WorkerProgressMonitor) MarkCompleted(queue string, failed bool) error {
	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	state, ok := monitor.queues[queue]
	if !ok {
		return errors.New("worker progress queue is not registered")
	}
	now := monitor.clock().UTC()
	state.active = false
	state.lastCompletedAt = now
	if failed {
		state.lastFailureAt = now
	}
	monitor.queues[queue] = state
	return nil
}

func (monitor *WorkerProgressMonitor) Run(ctx context.Context) {
	monitor.Emit(ctx)
	ticker := time.NewTicker(monitor.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			monitor.Emit(ctx)
		}
	}
}

func (monitor *WorkerProgressMonitor) Emit(ctx context.Context) {
	now := monitor.clock().UTC()
	monitor.mu.RLock()
	reports := make([]WorkerProgressReport, 0, workerQueueCount)
	for _, queue := range workerQueues {
		state := monitor.queues[queue]
		progressAnchor := state.lastCompletedAt
		if state.active {
			progressAnchor = state.lastStartedAt
		}
		report := WorkerProgressReport{
			Queue:           queue,
			HeartbeatAt:     now,
			LastStartedAt:   state.lastStartedAt,
			LastCompletedAt: state.lastCompletedAt,
			ProgressAge:     nonNegativeDuration(now.Sub(progressAnchor)),
			LastItemHash:    state.lastItemHash,
			Active:          state.active,
		}
		if !state.lastFailureAt.IsZero() {
			failureAge := nonNegativeDuration(now.Sub(state.lastFailureAt))
			report.FailureAge = &failureAge
		}
		reports = append(reports, report)
	}
	monitor.mu.RUnlock()

	for _, report := range reports {
		monitor.observer.ObserveWorkerProgress(ctx, report)
	}
}

func hashWorkerItem(itemID string) string {
	if itemID == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(itemID))
	return hex.EncodeToString(digest[:8])
}

func nonNegativeDuration(value time.Duration) time.Duration {
	if value < 0 {
		return 0
	}
	return value
}
