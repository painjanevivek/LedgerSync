package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ValidatePilotCurrency refuses startup when any provisioned tenant policy,
// account, or transfer is outside the selected one-currency pilot boundary.
func ValidatePilotCurrency(ctx context.Context, database *sql.DB, currency string) error {
	var tenantsWithoutPolicy, mismatchedPolicies, mismatchedAccounts, mismatchedTransfers int64
	err := database.QueryRowContext(ctx, `
SELECT
 (SELECT count(*) FROM tenants t LEFT JOIN tenant_transfer_policies p ON p.tenant_id=t.id WHERE p.tenant_id IS NULL),
 (SELECT count(*) FROM tenant_transfer_policies WHERE currency<>$1),
 (SELECT count(*) FROM accounts WHERE currency<>$1),
 (SELECT count(*) FROM transfers WHERE currency<>$1)`, currency).Scan(&tenantsWithoutPolicy, &mismatchedPolicies, &mismatchedAccounts, &mismatchedTransfers)
	if err != nil {
		return fmt.Errorf("validate pilot currency: %w", err)
	}
	if tenantsWithoutPolicy+mismatchedPolicies+mismatchedAccounts+mismatchedTransfers > 0 {
		return fmt.Errorf("pilot currency %s violated: missing_policies=%d mismatched_policies=%d mismatched_accounts=%d mismatched_transfers=%d", currency, tenantsWithoutPolicy, mismatchedPolicies, mismatchedAccounts, mismatchedTransfers)
	}
	return nil
}
