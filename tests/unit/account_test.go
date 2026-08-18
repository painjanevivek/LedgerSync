package unit_test

import (
	"errors"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/account"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

func TestDebitRequiresAccountOwnerPermission(t *testing.T) {
	acct, err := account.New("acct_1", "tenant_1", "USD", []account.Owner{{SubjectID: "reader", Permission: account.PermissionRead}}, time.Now())
	if err != nil {
		t.Fatalf("new account: %v", err)
	}
	amount, _ := money.New("USD", 1)
	if err := acct.ValidateDebit("reader", amount); !errors.Is(err, account.ErrUnauthorized) {
		t.Fatalf("error = %v, want denied", err)
	}
}

func TestCreditRequiresActiveMatchingCurrencyAccount(t *testing.T) {
	acct, _ := account.New("acct_1", "tenant_1", "USD", []account.Owner{{SubjectID: "owner", Permission: account.PermissionDebit}}, time.Now())
	amount, _ := money.New("INR", 1)
	if err := acct.ValidateCredit(amount); !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Fatalf("error = %v, want currency mismatch", err)
	}
}
