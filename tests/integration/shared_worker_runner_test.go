package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/bootstrap"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	"github.com/redis/go-redis/v9"
)

type sharedRunnerProgress struct {
	reports []observability.WorkerProgressReport
}

func (p *sharedRunnerProgress) ObserveWorkerProgress(_ context.Context, report observability.WorkerProgressReport) {
	p.reports = append(p.reports, report)
}

func TestSharedWorkerRunnerPreservesPublicationProjectionAndProgress(t *testing.T) {
	address := os.Getenv("LEDGERSYNC_TEST_REDIS_ADDR")
	if address == "" {
		t.Skip("isolated Redis address is required")
	}
	transfers, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// A separate logical database keeps this wiring test out of other cache fixtures.
	client := redis.NewClient(&redis.Options{Addr: address, DB: 14})
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = client.FlushDB(cleanup).Err()
		_ = client.Close()
	})
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := transfers.Submit(ctx, transferCommand(t, "shared-runner-original-001", "1.00")); err != nil {
		t.Fatal(err)
	}
	worker := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_worker")
	worker.db.SetMaxOpenConns(1)
	progress := &sharedRunnerProgress{}
	monitor, err := observability.NewWorkerProgressMonitor(progress, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	options := bootstrap.WorkerRunnerOptions{NodeRole: "worker",
		ItemObserver: func(queue string) func(string) {
			return func(id string) {
				if err := monitor.MarkItem(queue, id); err != nil {
					t.Error(err)
				}
			}
		},
		OnStarted: func(queue string) {
			if err := monitor.MarkStarted(queue, ""); err != nil {
				t.Error(err)
			}
		},
		OnCompleted: func(queue string, err error) {
			if err := monitor.MarkCompleted(queue, err != nil); err != nil {
				t.Error(err)
			}
		},
	}
	runner, err := bootstrap.NewWorkerRunnerWithOptions(ctx, config.Config{Environment: "development", NodeID: "r04-shared-runner", RedisStreamMaxLength: 1000}, worker.db, client, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	counts, err := runner.RunOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if counts.Outbox != 3 || counts.BalanceProjections != 3 || counts.WebhookDeliveries != 0 || counts.WebhookVerifications != 0 || counts.HoldExpirations != 0 {
		t.Fatalf("shared runner counts=%+v", counts)
	}
	monitor.Emit(ctx)
	if len(progress.reports) != 5 {
		t.Fatalf("progress reports=%d", len(progress.reports))
	}
	queues := make(map[string]bool)
	for _, report := range progress.reports {
		if queues[report.Queue] {
			t.Fatalf("duplicate queue progress: %s", report.Queue)
		}
		queues[report.Queue] = true
		if !report.HasCompleted || report.LastRunFailed {
			t.Fatalf("queue progress incomplete: %s", report.Queue)
		}
	}
	if !queues[observability.WorkerQueueHoldExpiry] {
		t.Fatal("reservation expiry progress missing")
	}
	if err := runner.ObserveHealth(ctx); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM outbox_publication_attempts WHERE status='published' AND worker_id LIKE 'ledgersync/worker/r04-shared-runner-%'`); got != 3 {
		t.Fatalf("publication attempts=%d", got)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM logical_message_receipts WHERE node_id='ledgersync/worker/r04-shared-runner'`); got != 3 {
		t.Fatalf("logical receipts=%d", got)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM ledger_postings`); got != 2 {
		t.Fatal("worker changed financial postings")
	}
}
