package integration_test

import (
	"context"
	"testing"
)

func TestProvisionedDatabaseRolesHaveNoForbiddenStandingAuthority(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	var rolesExist bool
	if err := database.QueryRowContext(context.Background(), `SELECT to_regrole('ledgersync_api') IS NOT NULL AND to_regrole('ledgersync_provisioning') IS NOT NULL AND to_regrole('ledgersync_support_readonly') IS NOT NULL AND to_regrole('ledgersync_break_glass') IS NOT NULL`).Scan(&rolesExist); err != nil {
		t.Fatal(err)
	}
	if !rolesExist {
		t.Skip("database group roles are provisioned by deploy/postgres/roles.sql")
	}
	checks := []struct {
		role, table, privilege string
		expected               bool
	}{
		{"ledgersync_api", "transfers", "INSERT", true},
		{"ledgersync_api", "api_rate_limit_windows", "SELECT", true},
		{"ledgersync_api", "transfer_velocity_events", "DELETE", true},
		{"ledgersync_api", "transfer_velocity_totals", "UPDATE", true},
		{"ledgersync_api", "delivery_attempts", "SELECT", true},
		{"ledgersync_api", "ledger_postings", "SELECT", true},
		{"ledgersync_api", "reconciliation_runs", "SELECT", true},
		{"ledgersync_api", "ledger_postings", "DELETE", false},
		{"ledgersync_worker", "ledger_postings", "INSERT", false},
		{"ledgersync_reconciliation", "reconciliation_runs", "INSERT", true},
		{"ledgersync_provisioning", "partner_provisioning_requests", "INSERT", true},
		{"ledgersync_provisioning", "ledger_postings", "INSERT", false},
		{"ledgersync_api", "partner_credential_events", "INSERT", false},
		{"ledgersync_support_readonly", "audit_events", "SELECT", true},
		{"ledgersync_support_readonly", "audit_events", "UPDATE", false},
		{"ledgersync_break_glass", "transfers", "SELECT", false},
	}
	for _, check := range checks {
		var allowed bool
		if err := database.QueryRowContext(context.Background(), `SELECT has_table_privilege($1,$2,$3)`, check.role, check.table, check.privilege).Scan(&allowed); err != nil {
			t.Fatal(err)
		}
		if allowed != check.expected {
			t.Errorf("%s %s on %s=%t, want %t", check.role, check.privilege, check.table, allowed, check.expected)
		}
	}
}
