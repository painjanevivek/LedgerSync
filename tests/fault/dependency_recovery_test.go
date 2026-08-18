package fault_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestExpiredOutboxLeaseIsRecoveredAfterWorkerLoss(t *testing.T) {
	service, database, _ := requireFaultDependencies(t, 10_000)
	if _, err := service.Submit(context.Background(), faultTransferCommand(t, "fault-outbox-recovery-key-0001", "25.00")); err != nil { t.Fatal(err) }
	now := time.Now().UTC().Add(time.Second)
	repository, err := db.NewOutboxRepository(database, func() time.Time { return now }); if err != nil { t.Fatal(err) }
	claimed, err := repository.Claim(context.Background(), "crashed-worker", 10, time.Second); if err != nil { t.Fatal(err) }
	if len(claimed) != 2 { t.Fatalf("claimed=%d, want 2 account events", len(claimed)) }
	now = now.Add(2 * time.Second)
	recovered, err := repository.Claim(context.Background(), "recovery-worker", 10, time.Second); if err != nil { t.Fatal(err) }
	if len(recovered) != 2 { t.Fatalf("recovered=%d, want 2 expired events", len(recovered)) }
	for _, event := range recovered { if err := repository.MarkPublished(context.Background(), "recovery-worker", event.ID, now); err != nil { t.Fatal(err) } }
	var unpublished int
	if err := database.QueryRowContext(context.Background(), `SELECT count(*) FROM outbox_events WHERE published_at IS NULL AND dead_at IS NULL`).Scan(&unpublished); err != nil { t.Fatal(err) }
	if unpublished != 0 { t.Fatalf("unpublished recovery events=%d, want 0", unpublished) }
}

func TestRedisDependencyLossReschedulesWithoutChangingFinancialRecords(t *testing.T) {
	service, database, _ := requireFaultDependencies(t, 10_000)
	if _, err := service.Submit(context.Background(), faultTransferCommand(t, "fault-dependency-loss-key-01", "25.00")); err != nil { t.Fatal(err) }
	now := time.Now().UTC().Add(time.Second)
	repository, err := db.NewOutboxRepository(database, func() time.Time { return now }); if err != nil { t.Fatal(err) }
	worker, err := outbox.NewWorker(repository, unavailablePublisher{}, nil, func() time.Time { return now }, outbox.Config{WorkerID: "dependency-loss-worker", MaxAttempts: 3}); if err != nil { t.Fatal(err) }
	if _, err := worker.RunOnce(context.Background()); err != nil { t.Fatal(err) }
	var unpublished, published, postings int
	if err := database.QueryRowContext(context.Background(), `SELECT count(*) FROM outbox_events WHERE published_at IS NULL`).Scan(&unpublished); err != nil { t.Fatal(err) }
	if err := database.QueryRowContext(context.Background(), `SELECT count(*) FROM outbox_events WHERE published_at IS NOT NULL`).Scan(&published); err != nil { t.Fatal(err) }
	if err := database.QueryRowContext(context.Background(), `SELECT count(*) FROM ledger_postings`).Scan(&postings); err != nil { t.Fatal(err) }
	if unpublished != 2 || published != 0 || postings != 2 { t.Fatalf("unpublished=%d published=%d postings=%d; expected 2,0,2", unpublished, published, postings) }
}

type unavailablePublisher struct{}
func (unavailablePublisher) Publish(context.Context, outbox.Event) error { return errors.New("redis unavailable") }
