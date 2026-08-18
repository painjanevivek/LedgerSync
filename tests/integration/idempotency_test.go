package integration_test

import (
	"context"
	"errors"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
)

func TestTransferIdempotencyReplaysOriginalOutcomeWithoutSecondMovement(t *testing.T) {
	service, database := requireTransferService(t, 10000)
	first, err := service.Submit(context.Background(), transferCommand(t, "integration-idempotency-key-0001", "25.00"))
	if err != nil {
		t.Fatalf("first submission: %v", err)
	}
	second, err := service.Submit(context.Background(), transferCommand(t, "integration-idempotency-key-0001", "25.00"))
	if err != nil {
		t.Fatalf("replay submission: %v", err)
	}
	if first.Replayed || !second.Replayed || first.Result.TransferID != second.Result.TransferID {
		t.Fatalf("unexpected replay results: first=%#v second=%#v", first, second)
	}
	if countRows(t, database, `SELECT count(*) FROM transfers`) != 1 || countRows(t, database, `SELECT count(*) FROM ledger_postings`) != 2 || countRows(t, database, `SELECT count(*) FROM outbox_events`) != 2 {
		t.Fatal("idempotent replay created additional financial side effects")
	}
}

func TestTransferIdempotencyRejectsSameKeyWithDifferentFinancialIntent(t *testing.T) {
	service, database := requireTransferService(t, 10000)
	if _, err := service.Submit(context.Background(), transferCommand(t, "integration-idempotency-key-0002", "25.00")); err != nil {
		t.Fatalf("first submission: %v", err)
	}
	_, err := service.Submit(context.Background(), transferCommand(t, "integration-idempotency-key-0002", "30.00"))
	if !errors.Is(err, transfers.ErrIdempotencyConflict) {
		t.Fatalf("error = %v, want idempotency conflict", err)
	}
	if countRows(t, database, `SELECT count(*) FROM transfers`) != 1 || countRows(t, database, `SELECT count(*) FROM ledger_postings`) != 2 {
		t.Fatal("mismatched idempotency request changed financial state")
	}
}
