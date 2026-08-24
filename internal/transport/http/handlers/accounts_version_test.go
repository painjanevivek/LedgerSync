package handlers

import (
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
)

func TestAccountReadDTOKeepsConfigurationAndBalanceVersionsDistinct(t *testing.T) {
	response := mapAccountResponse(accounts.Summary{
		AccountID: "account-1", AccountVersion: 7,
		Balance: accounts.Balance{Version: 42},
	})
	if response.AccountVersion != "7" || response.Version != "42" {
		t.Fatalf("account_version=%q balance version=%q", response.AccountVersion, response.Version)
	}
}
