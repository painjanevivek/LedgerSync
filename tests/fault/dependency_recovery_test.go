package fault_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestExpiredOutboxLeaseIsRecoveredAfterWorkerLoss(t *testing.T) {
	service, database, _ := requireFaultDependencies(t, 10_000)
	if _, err := service.Submit(context.Background(), faultTransferCommand(t, "fault-outbox-recovery-key-0001", "25.00")); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(time.Second)
	repository, err := db.NewOutboxRepository(database, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := repository.Claim(context.Background(), "crashed-worker", 10, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 3 {
		t.Fatalf("claimed=%d, want 3 transfer events", len(claimed))
	}
	now = now.Add(2 * time.Second)
	recovered, err := repository.Claim(context.Background(), "recovery-worker", 10, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 3 {
		t.Fatalf("recovered=%d, want 3 expired events", len(recovered))
	}
	for _, event := range recovered {
		if err := repository.MarkPublished(context.Background(), "recovery-worker", event.ID, now); err != nil {
			t.Fatal(err)
		}
	}
	var unpublished int
	if err := database.QueryRowContext(context.Background(), `SELECT count(*) FROM outbox_events WHERE published_at IS NULL AND dead_at IS NULL`).Scan(&unpublished); err != nil {
		t.Fatal(err)
	}
	if unpublished != 0 {
		t.Fatalf("unpublished recovery events=%d, want 0", unpublished)
	}
}

func TestRedisDependencyLossReschedulesWithoutChangingFinancialRecords(t *testing.T) {
	service, database, _ := requireFaultDependencies(t, 10_000)
	if _, err := service.Submit(context.Background(), faultTransferCommand(t, "fault-dependency-loss-key-01", "25.00")); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(time.Second)
	repository, err := db.NewOutboxRepository(database, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	worker, err := outbox.NewWorker(repository, unavailablePublisher{}, nil, func() time.Time { return now }, outbox.Config{WorkerID: "dependency-loss-worker", MaxAttempts: 3})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	var unpublished, published, postings int
	if err := database.QueryRowContext(context.Background(), `SELECT count(*) FROM outbox_events WHERE published_at IS NULL`).Scan(&unpublished); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(context.Background(), `SELECT count(*) FROM outbox_events WHERE published_at IS NOT NULL`).Scan(&published); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(context.Background(), `SELECT count(*) FROM ledger_postings`).Scan(&postings); err != nil {
		t.Fatal(err)
	}
	if unpublished != 3 || published != 0 || postings != 2 {
		t.Fatalf("unpublished=%d published=%d postings=%d; expected 3,0,2", unpublished, published, postings)
	}
}

func TestReconciliationRecordsAnAlertForProjectionMismatch(t *testing.T) {
	serviceForTransfer, database, _ := requireFaultDependencies(t, 10_000)
	if _, err := serviceForTransfer.Submit(context.Background(), faultTransferCommand(t, "fault-reconcile-mismatch-key", "25.00")); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(context.Background(), `UPDATE account_balance_projections SET ledger_minor = ledger_minor + 1 WHERE account_id = $1`, faultSourceID); err != nil {
		t.Fatal(err)
	}
	repository, err := db.NewReconciliationRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	service, err := reconciliation.NewService(repository, func() time.Time { return time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Run(context.Background(), faultTenantID)
	if err != nil {
		t.Fatal(err)
	}
	if result.MismatchCount == 0 || result.Status != reconciliation.StatusMismatch {
		t.Fatalf("result=%#v, want persisted mismatch", result)
	}
	var persisted int
	if err := database.QueryRowContext(context.Background(), `SELECT count(*) FROM reconciliation_runs WHERE tenant_id = $1 AND status = 'mismatch'`, faultTenantID).Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	if persisted != 1 {
		t.Fatalf("persisted mismatch runs=%d, want 1", persisted)
	}
}

type unavailablePublisher struct{}

func (unavailablePublisher) Publish(context.Context, outbox.Event) error {
	return errors.New("redis unavailable")
}
