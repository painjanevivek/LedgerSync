package integration_test

import (
	"context"
	"testing"
)

func TestMigrationsAreForwardCompatibleAndPreserveExistingReadContracts(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	var versions int
	if err := database.QueryRowContext(context.Background(), `SELECT count(*) FROM schema_migrations`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != 11 {
		t.Fatalf("migration versions=%d, want 11", versions)
	}
	for _, table := range []string{"accounts", "account_credit_permissions", "ledger_postings", "outbox_events", "reconciliation_runs", "reconciliation_mismatches", "delivery_attempts", "delivery_replay_actions", "tenant_transfer_policies", "api_rate_limit_windows", "account_opening_balances", "retention_runs", "outbox_replay_actions", "partner_provisioning_requests", "partner_credential_events"} {
		var exists bool
		if err := database.QueryRowContext(context.Background(), `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("required table %s is missing after migration", table)
		}
	}
	var columns int
	if err := database.QueryRowContext(context.Background(), `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = 'public'
  AND (table_name, column_name) IN (
    ('accounts', 'tenant_id'),
    ('account_balance_projections', 'available_minor'),
    ('account_balance_projections', 'ledger_minor'),
    ('ledger_postings', 'direction'),
    ('outbox_events', 'aggregate_version')
  )`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != 5 {
		t.Fatalf("legacy financial read contract columns=%d, want 5", columns)
	}
}
