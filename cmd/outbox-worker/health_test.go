package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
)

type discardProgress struct{}

func TestWorkerHealthPathCanBeIsolatedPerProcess(t *testing.T) {
	t.Setenv("LEDGERSYNC_WORKER_HEALTH_PATH", "")
	if configuredWorkerHealthPath() != workerHealthPath {
		t.Fatal("default probe path changed")
	}
	private := filepath.Join(t.TempDir(), "health.json")
	t.Setenv("LEDGERSYNC_WORKER_HEALTH_PATH", private)
	if configuredWorkerHealthPath() != private {
		t.Fatal("private process health path ignored")
	}
}

func (discardProgress) ObserveWorkerProgress(context.Context, observability.WorkerProgressReport) {}

func TestWorkerHealthRequiresSuccessfulPollsAndDetectsFailureAndStall(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "health.json")
	observer := &workerHealthObserver{delegate: discardProgress{}, path: path, reports: make(map[string]observability.WorkerProgressReport)}
	monitor, err := observability.NewWorkerProgressMonitor(observer, time.Second, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	emitAndCheck := func(wantReady bool) {
		t.Helper()
		monitor.Emit(context.Background())
		if got := checkWorkerHealth(path, now) == nil; got != wantReady {
			t.Fatalf("ready=%v want=%v", got, wantReady)
		}
	}
	emitAndCheck(false) // Process heartbeat alone, before any queue poll.
	queues := []string{observability.WorkerQueueOutbox, observability.WorkerQueueWebhookDelivery, observability.WorkerQueueWebhookVerification, observability.WorkerQueueProjection}
	for _, queue := range queues {
		if err := monitor.MarkCompleted(queue, false); err != nil {
			t.Fatal(err)
		}
	}
	emitAndCheck(true) // Empty queues are healthy after successful polls.
	if err := checkWorkerHealth(path, now.Add(workerHeartbeatLimit+time.Nanosecond)); err == nil {
		t.Fatal("stale heartbeat accepted")
	}
	if err := checkWorkerHealth(path, now.Add(-time.Second)); err == nil {
		t.Fatal("future heartbeat accepted")
	}
	now = now.Add(time.Second)
	if err := monitor.MarkCompleted(observability.WorkerQueueOutbox, true); err != nil {
		t.Fatal(err)
	}
	emitAndCheck(false) // Live process, failed database claim.
	if err := monitor.MarkCompleted(observability.WorkerQueueOutbox, false); err != nil {
		t.Fatal(err)
	}
	emitAndCheck(true) // Recovery needs a new successful poll.
	if err := monitor.MarkStarted(observability.WorkerQueueProjection, "private-item"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(workerProgressLimit + time.Second)
	emitAndCheck(false) // Monitor still emits; the queue loop is stalled.
}

func TestWorkerHealthFailsClosedOnMissingAndMalformedEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "health.json")
	if err := checkWorkerHealth(path, time.Now()); err == nil {
		t.Fatal("missing file accepted")
	}
	for _, body := range []string{"{", "{}", `{"ready":true}`, string(make([]byte, 1025))} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if err := checkWorkerHealth(path, time.Now()); err == nil {
			t.Fatalf("invalid evidence accepted: %q", body)
		}
	}
}
