package integration_test

import (
	"context"
	"sync"
	"testing"
)

func TestCompetingTransfersNeverOverdrawSourceAccount(t *testing.T) {
	service, database := requireTransferService(t, 10000)
	commands := []struct{ key, amount string }{{"concurrent-transfer-key-001", "70.00"}, {"concurrent-transfer-key-002", "60.00"}}
	errs := make(chan error, len(commands))
	var group sync.WaitGroup
	for _, item := range commands {
		item := item
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := service.Submit(context.Background(), transferCommand(t, item.key, item.amount))
			errs <- err
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent submission returned error: %v", err)
		}
	}
	var available int64
	if err := database.QueryRow(`SELECT available_minor FROM account_balance_projections WHERE account_id = $1`, testSourceID).Scan(&available); err != nil {
		t.Fatal(err)
	}
	if available < 0 {
		t.Fatalf("source was overdrawn: %d", available)
	}
	if countRows(t, database, `SELECT count(*) FROM transfers WHERE status = 'posted'`) != 1 || countRows(t, database, `SELECT count(*) FROM ledger_postings`) != 2 {
		t.Fatal("competing transfers did not produce exactly one balanced movement")
	}
}
