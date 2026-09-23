package observability

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

type progressCapture struct {
	reports chan WorkerProgressReport
}

func (capture *progressCapture) ObserveWorkerProgress(_ context.Context, report WorkerProgressReport) {
	capture.reports <- report
}

type lockedClock struct {
	mu  sync.Mutex
	now time.Time
}

func (clock *lockedClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *lockedClock) Advance(duration time.Duration) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.now = clock.now.Add(duration)
}

func TestWorkerProgressMonitorReportsStallOutsideBlockedWork(t *testing.T) {
	clock := &lockedClock{now: time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)}
	capture := &progressCapture{reports: make(chan WorkerProgressReport, 64)}
	monitor, err := NewWorkerProgressMonitor(capture, 5*time.Millisecond, clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := monitor.MarkStarted(WorkerQueueWebhookDelivery, "delivery-sensitive-id"); err != nil {
		t.Fatal(err)
	}

	blocked := make(chan struct{})
	blockedWorkReturned := make(chan struct{})
	go func() {
		<-blocked
		close(blockedWorkReturned)
	}()
	clock.Advance(45 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go monitor.Run(ctx)

	deadline := time.After(time.Second)
	for {
		select {
		case report := <-capture.reports:
			if report.Queue != WorkerQueueWebhookDelivery || !report.Active {
				continue
			}
			if report.ProgressAge < 45*time.Second {
				t.Fatalf("progress age=%s want at least 45s", report.ProgressAge)
			}
			if !report.HeartbeatAt.Equal(clock.Now()) {
				t.Fatalf("heartbeat=%s want=%s", report.HeartbeatAt, clock.Now())
			}
			if report.LastItemHash == "" || strings.Contains(report.LastItemHash, "delivery-sensitive-id") {
				t.Fatalf("item hash is missing or exposes the raw identifier: %q", report.LastItemHash)
			}
			close(blocked)
			select {
			case <-blockedWorkReturned:
			case <-time.After(time.Second):
				t.Fatal("blocked work did not exit")
			}
			return
		case <-deadline:
			t.Fatal("independent progress monitor did not report the stalled queue")
		}
	}
}

func TestWorkerProgressMonitorRecordsCompletionAndFailure(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	capture := &progressCapture{reports: make(chan WorkerProgressReport, 16)}
	monitor, err := NewWorkerProgressMonitor(capture, time.Second, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if err := monitor.MarkStarted(WorkerQueueOutbox, "event-1"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if err := monitor.MarkCompleted(WorkerQueueOutbox, true); err != nil {
		t.Fatal(err)
	}
	now = now.Add(3 * time.Second)
	monitor.Emit(context.Background())

	var report WorkerProgressReport
	for index := 0; index < workerQueueCount; index++ {
		candidate := <-capture.reports
		if candidate.Queue == WorkerQueueOutbox {
			report = candidate
		}
	}
	if report.Active {
		t.Fatal("completed queue remained active")
	}
	if report.ProgressAge != 3*time.Second {
		t.Fatalf("progress age=%s want=3s", report.ProgressAge)
	}
	if report.FailureAge == nil || *report.FailureAge != 3*time.Second {
		t.Fatalf("failure age=%v want=3s", report.FailureAge)
	}
}

func TestWorkerProgressMonitorRejectsUnknownQueue(t *testing.T) {
	capture := &progressCapture{reports: make(chan WorkerProgressReport, 1)}
	monitor, err := NewWorkerProgressMonitor(capture, time.Second, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := monitor.MarkStarted("tenant-controlled-queue", "item"); err == nil {
		t.Fatal("expected unknown queue to be rejected")
	}
}
