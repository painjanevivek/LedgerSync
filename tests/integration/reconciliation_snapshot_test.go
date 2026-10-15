package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestReconciliationRecordsOutboxInItsLedgerSnapshot(t *testing.T) {
	service, database := requireTransferService(t, 10000)
	if _, err := service.Submit(context.Background(), transferCommand(t, "snapshot-outbox-initial", "1.00")); err != nil {
		t.Fatal(err)
	}
	repository, _ := db.NewReconciliationRepository(database)
	result, err := repository.Reconcile(context.Background(), testTenantID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := database.QueryRow(`SELECT details->'outbox_snapshot' FROM reconciliation_runs WHERE id=$1`, result.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(raw, &snapshot); err != nil || snapshot["isolation"] != "repeatable_read" || snapshot["sampled_at"] == nil || snapshot["pending_count"] == nil {
		t.Fatalf("outbox snapshot unavailable: %s, %v", raw, err)
	}
	var group sync.WaitGroup
	errors := make(chan error, 1)
	group.Add(1)
	go func() {
		defer group.Done()
		for i := 0; i < 10; i++ {
			_, err := service.Submit(context.Background(), transferCommand(t, fmt.Sprintf("snapshot-concurrent-%04d", i), "1.00"))
			if err != nil {
				errors <- err
				return
			}
		}
	}()
	for i := 0; i < 5; i++ {
		r, err := repository.Reconcile(context.Background(), testTenantID, time.Now().UTC())
		if err != nil || r.MismatchCount != 0 || r.LedgerWatermark == "" {
			t.Fatalf("concurrent reconciliation: %#v, %v", r, err)
		}
	}
	group.Wait()
	select {
	case err := <-errors:
		t.Fatal(err)
	default:
	}
}
