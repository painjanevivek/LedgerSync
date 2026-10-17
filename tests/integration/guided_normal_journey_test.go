package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/events"
	"github.com/redis/go-redis/v9"
)

func TestPortfolioNormalJourney(t *testing.T) {
	_, admin := requireTransferService(t, 10000)
	harness := RequireHarness(t)
	requireWorkloadRoles(t, admin)
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	workerLogin := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_worker")
	repository, err := db.NewTransferRepository(api.db, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	service, err := transfers.NewService(repository, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := service.Submit(ctx, transferCommand(t, "guided-normal-original-intent", "1.00"))
	if err != nil || result.Result.Status != "posted" {
		t.Fatalf("normal command failed: %v", err)
	}
	if countRows(t, admin, `SELECT count(*) FROM ledger_postings`) != 2 || countRows(t, admin, `SELECT count(*) FROM journal_transactions`) != 1 {
		t.Fatal("normal command is not one balanced journal")
	}
	client := redis.NewClient(&redis.Options{Addr: harness.RedisAddr})
	defer func() { _ = client.Close() }()
	stream, err := events.NewRedisStreams(client, "guided:normal:"+result.Result.TransferID)
	if err != nil {
		t.Fatal(err)
	}
	store, err := db.NewOutboxRepository(workerLogin.db, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := outbox.NewWorker(store, stream, nil, time.Now, outbox.Config{WorkerID: "guided-normal-worker", BatchSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if count, err := worker.RunOnce(ctx); err != nil || count != 3 {
		t.Fatalf("normal notification delivery: count=%d err=%v", count, err)
	}
	if countRows(t, admin, `SELECT count(*) FROM outbox_events WHERE published_at IS NOT NULL`) != 3 {
		t.Fatal("normal notifications did not record publication")
	}
	reconciler, err := db.NewReconciliationRepository(admin)
	if err != nil {
		t.Fatal(err)
	}
	check, err := reconciler.Reconcile(ctx, testTenantID, time.Now())
	if err != nil || check.MismatchCount != 0 {
		t.Fatalf("normal financial reconciliation failed: %v", err)
	}
}
