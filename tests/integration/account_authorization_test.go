package integration_test

import (
	"context"
	"errors"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transactions"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestAccountReadsDenyUnownedAccountsWithoutDisclosure(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	balances, err := db.NewBalanceRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := balances.ReadCurrent(ctx, testTenantID, "different-subject", testSourceID); !errors.Is(err, db.ErrBalanceNotAuthorized) {
		t.Fatalf("expected safe authorization denial, got %v", err)
	}
	historyRepo, err := db.NewTransactionHistoryRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := historyRepo.ListAccountHistory(ctx, testTenantID, "different-subject", testSourceID, "", 10); !errors.Is(err, transactions.ErrHistoryNotFound) {
		t.Fatalf("expected safe history denial, got %v", err)
	}
}

func TestAccountListContainsOnlyAccountsOwnedByCaller(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	repository, err := db.NewAccountRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := repository.ListOwned(context.Background(), testTenantID, testActorID)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].AccountID != testSourceID {
		t.Fatalf("owned account list disclosed unexpected account: %#v", accounts)
	}
}
