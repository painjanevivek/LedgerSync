package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
)

const workerHealthPath = "/tmp/ledgersync-worker-health.json"

func configuredWorkerHealthPath() string {
	if configured := strings.TrimSpace(os.Getenv("LEDGERSYNC_WORKER_HEALTH_PATH")); configured != "" {
		return configured
	}
	return workerHealthPath
}

const workerProgressLimit = time.Minute
const workerHeartbeatLimit = 15 * time.Second

type workerHealthSnapshot struct {
	ObservedAt time.Time `json:"observed_at"`
	Ready      bool      `json:"ready"`
}

// The existing monitor runs independently of the queue loop. A live process
// cannot mask a failed poll, an unstarted queue, or a stalled active item.
type workerHealthObserver struct {
	delegate observability.WorkerProgressObserver
	path     string
	reports  map[string]observability.WorkerProgressReport
}

func (observer *workerHealthObserver) ObserveWorkerProgress(ctx context.Context, report observability.WorkerProgressReport) {
	observer.delegate.ObserveWorkerProgress(ctx, report)
	observer.reports[report.Queue] = report
	// Emit writes once after all five ordered reports, including expiry.
	if report.Queue != observability.WorkerQueueHoldExpiry {
		return
	}
	ready := len(observer.reports) == 5
	for _, queue := range observer.reports {
		if !queue.HasCompleted || queue.LastRunFailed || queue.ProgressAge > workerProgressLimit {
			ready = false
		}
	}
	encoded, err := json.Marshal(workerHealthSnapshot{ObservedAt: report.HeartbeatAt, Ready: ready})
	if err == nil {
		err = os.WriteFile(observer.path+".pending", encoded, 0600)
	}
	if err == nil {
		err = os.Rename(observer.path+".pending", observer.path)
	}
	if err != nil {
		// The previous heartbeat expires. Never create healthy evidence on failure.
		fmt.Fprintln(os.Stderr, "worker readiness evidence write failed:", err)
	}
}

func checkWorkerHealth(path string, now time.Time) error {
	info, err := os.Stat(path)
	if err != nil || info.Size() > 1024 {
		return errors.New("worker readiness evidence unavailable")
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return errors.New("worker readiness evidence unreadable")
	}
	var snapshot workerHealthSnapshot
	if err := json.Unmarshal(encoded, &snapshot); err != nil || snapshot.ObservedAt.IsZero() {
		return errors.New("worker readiness evidence invalid")
	}
	age := now.Sub(snapshot.ObservedAt)
	if !snapshot.Ready || age < 0 || age > workerHeartbeatLimit {
		return errors.New("worker queue polls failed, stalled, or not yet verified")
	}
	return nil
}
