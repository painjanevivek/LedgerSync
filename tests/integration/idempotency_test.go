package integration_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
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
	if countRows(t, database, `SELECT count(*) FROM transfers`) != 1 || countRows(t, database, `SELECT count(*) FROM ledger_postings`) != 2 || countRows(t, database, `SELECT count(*) FROM outbox_events`) != 3 {
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

func TestConcurrentSameKeyRequestsResolveToOneMovement(t *testing.T) {
	service, database := requireTransferService(t, 100_000)
	const attempts = 12
	type outcome struct {
		transferID string
		replayed   bool
		err        error
	}
	outcomes := make(chan outcome, attempts)
	var wait sync.WaitGroup
	for range attempts {
		wait.Add(1)
		go func() {
			defer wait.Done()
			submission, err := service.Submit(context.Background(), transferCommand(t, "concurrent-same-key-0001", "25.00"))
			outcomes <- outcome{transferID: submission.Result.TransferID, replayed: submission.Replayed, err: err}
		}()
	}
	wait.Wait()
	close(outcomes)

	transferID := ""
	originals := 0
	for result := range outcomes {
		if result.err != nil {
			t.Fatalf("same-key request: %v", result.err)
		}
		if transferID == "" {
			transferID = result.transferID
		}
		if result.transferID != transferID {
			t.Fatalf("multiple transfer IDs: %s and %s", transferID, result.transferID)
		}
		if !result.replayed {
			originals++
		}
	}
	if originals != 1 {
		t.Fatalf("original outcomes=%d, want 1", originals)
	}
	if countRows(t, database, `SELECT count(*) FROM transfers`) != 1 || countRows(t, database, `SELECT count(*) FROM journal_transactions`) != 1 || countRows(t, database, `SELECT count(*) FROM ledger_postings`) != 2 {
		t.Fatal("concurrent same-key requests created more than one movement")
	}
}

func TestReplaySurvivesLostResponseAndServiceRestart(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	command := transferCommand(t, "restart-lost-response-key-0001", "25.00")
	first, err := service.Submit(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}

	fixedClock := func() time.Time { return time.Date(2026, 8, 18, 9, 15, 0, 0, time.UTC) }
	restartedRepository, err := db.NewTransferRepository(database, fixedClock)
	if err != nil {
		t.Fatal(err)
	}
	restartedService, err := transfers.NewService(restartedRepository, fixedClock)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := restartedService.Submit(context.Background(), command)
	if err != nil || !replay.Replayed || replay.Result.TransferID != first.Result.TransferID {
		t.Fatalf("restart replay=%#v err=%v", replay, err)
	}
	if countRows(t, database, `SELECT count(*) FROM transfers`) != 1 || countRows(t, database, `SELECT count(*) FROM ledger_postings`) != 2 {
		t.Fatal("lost-response retry after restart created a second movement")
	}
}

func TestIdempotencyKeyRejectsChangedAccountAndCurrencyIntent(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	key := "changed-intent-conflict-key-0001"
	if _, err := service.Submit(context.Background(), transferCommand(t, key, "25.00")); err != nil {
		t.Fatal(err)
	}

	changedSource := transferCommand(t, key, "25.00")
	changedSource.DebitAccountID = "00000000-0000-0000-0000-000000000099"
	changedDestination := transferCommand(t, key, "25.00")
	changedDestination.CreditAccountID = "00000000-0000-0000-0000-000000000098"
	changedCurrency := transferCommand(t, key, "25.00")
	changedCurrency.Amount, _ = money.Parse("INR", "25.00")

	for name, command := range map[string]transfers.Command{
		"source": changedSource, "destination": changedDestination, "currency": changedCurrency,
	} {
		if _, err := service.Submit(context.Background(), command); !errors.Is(err, transfers.ErrIdempotencyConflict) {
			t.Errorf("%s change error=%v, want idempotency conflict", name, err)
		}
	}
	if countRows(t, database, `SELECT count(*) FROM transfers`) != 1 || countRows(t, database, `SELECT count(*) FROM ledger_postings`) != 2 {
		t.Fatal("changed financial intent altered committed state")
	}
}
