// Package transfers contains use-case rules and ports; adapters live outside it.
package transfers

import (
	"fmt"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/account"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

// AuthorizeDebit is intentionally a pure predicate. The PostgreSQL adapter
// obtains the account through a tenant-scoped, locked query before calling it.
func AuthorizeDebit(principalSubjectID string, source account.Account, amount money.Money) error {
	if err := source.ValidateDebit(principalSubjectID, amount); err != nil {
		return fmt.Errorf("authorize debit: %w", err)
	}
	return nil
}

func AuthorizeCredit(destination account.Account, amount money.Money) error {
	if err := destination.ValidateCredit(amount); err != nil {
		return fmt.Errorf("authorize credit: %w", err)
	}
	return nil
}
