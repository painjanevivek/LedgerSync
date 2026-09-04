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
		{"ledgersync_api", "accounts", "INSERT", false},
		{"ledgersync_api", "accounts", "UPDATE", false},
		{"ledgersync_api", "account_opening_balances", "SELECT", true},
		{"ledgersync_api", "account_opening_balances", "INSERT", false},
		{"ledgersync_api", "account_balance_projections", "DELETE", false},
		{"ledgersync_api", "api_rate_limit_windows", "SELECT", true},
		{"ledgersync_api", "transfer_velocity_events", "DELETE", true},
		{"ledgersync_api", "transfer_velocity_totals", "UPDATE", true},
		{"ledgersync_api", "tenant_transfer_policies", "SELECT", true},
		{"ledgersync_api", "tenant_transfer_policies", "UPDATE", false},
		{"ledgersync_api", "delivery_attempts", "SELECT", true},
		{"ledgersync_api", "ledger_postings", "SELECT", true},
		{"ledgersync_api", "reconciliation_runs", "SELECT", true},
		{"ledgersync_api", "reconciliation_runs", "INSERT", true},
		{"ledgersync_api", "reconciliation_mismatches", "INSERT", true},
		{"ledgersync_api", "reconciliation_run_commands", "SELECT", true},
		{"ledgersync_api", "reconciliation_run_commands", "INSERT", true},
		{"ledgersync_api", "reconciliation_run_commands", "DELETE", true},
		{"ledgersync_api", "outbox_events", "SELECT", true},
		{"ledgersync_api", "outbox_events", "UPDATE", false},
		{"ledgersync_api", "audit_events", "SELECT", true},
		{"ledgersync_api", "audit_events", "INSERT", false},
		{"ledgersync_api", "schema_migrations", "SELECT", true},
		{"ledgersync_api", "operator_onboarding_preferences", "SELECT", true},
		{"ledgersync_api", "operator_onboarding_preferences", "INSERT", true},
		{"ledgersync_api", "operator_onboarding_preferences", "UPDATE", true},
		{"ledgersync_api", "operator_onboarding_preferences", "DELETE", false},
		{"ledgersync_api", "investigation_saved_views", "SELECT", true},
		{"ledgersync_api", "investigation_saved_views", "INSERT", true},
		{"ledgersync_api", "investigation_saved_views", "UPDATE", true},
		{"ledgersync_api", "investigation_saved_views", "DELETE", true},
		{"ledgersync_api", "investigation_workspaces", "SELECT", true},
		{"ledgersync_api", "investigation_workspaces", "INSERT", true},
		{"ledgersync_api", "investigation_workspaces", "UPDATE", true},
		{"ledgersync_api", "investigation_workspaces", "DELETE", false},
		{"ledgersync_api", "investigation_workspace_references", "SELECT", true},
		{"ledgersync_api", "investigation_workspace_references", "INSERT", true},
		{"ledgersync_api", "investigation_workspace_references", "UPDATE", false},
		{"ledgersync_api", "investigation_workspace_references", "DELETE", false},
		{"ledgersync_api", "funding_events", "SELECT", true},
		{"ledgersync_api", "funding_events", "INSERT", true},
		{"ledgersync_api", "funding_events", "UPDATE", true},
		{"ledgersync_api", "funding_events", "DELETE", false},
		{"ledgersync_api", "approval_records", "INSERT", true},
		{"ledgersync_api", "funding_velocity_events", "INSERT", true},
		{"ledgersync_api", "tenant_funding_policies", "SELECT", true},
		{"ledgersync_api", "tenant_funding_policies", "UPDATE", false},
		{"ledgersync_api", "ledger_postings", "DELETE", false},
		{"ledgersync_worker", "ledger_postings", "INSERT", false},
		{"ledgersync_worker", "transfers", "INSERT", false},
		{"ledgersync_worker", "accounts", "UPDATE", false},
		{"ledgersync_reconciliation", "reconciliation_runs", "INSERT", true},
		{"ledgersync_reconciliation", "audit_events", "INSERT", false},
		{"ledgersync_provisioning", "partner_provisioning_requests", "INSERT", true},
		{"ledgersync_provisioning", "accounts", "INSERT", false},
		{"ledgersync_provisioning", "accounts", "UPDATE", false},
		{"ledgersync_provisioning", "account_owners", "DELETE", false},
		{"ledgersync_provisioning", "ledger_postings", "INSERT", false},
		{"ledgersync_api", "partner_credential_events", "INSERT", false},
		{"ledgersync_api", "developer_webhook_endpoints", "SELECT", true},
		{"ledgersync_api", "developer_webhook_endpoints", "UPDATE", true},
		{"ledgersync_api", "developer_webhook_events", "DELETE", false},
		{"ledgersync_api", "bff_actor_assertion_replays", "SELECT", true},
		{"ledgersync_api", "bff_actor_assertion_replays", "INSERT", true},
		{"ledgersync_api", "bff_actor_assertion_replays", "DELETE", true},
		{"ledgersync_api", "bff_actor_assertion_replays", "UPDATE", false},
		{"ledgersync_api", "webhook_endpoint_verification_jobs", "INSERT", true},
		{"ledgersync_api", "webhook_endpoint_verification_jobs", "SELECT", false},
		{"ledgersync_worker", "developer_webhook_endpoints", "SELECT", true},
		{"ledgersync_worker", "developer_webhook_endpoints", "UPDATE", false},
		{"ledgersync_worker", "webhook_endpoint_verification_jobs", "SELECT", true},
		{"ledgersync_worker", "webhook_endpoint_verification_jobs", "UPDATE", true},
		{"ledgersync_worker", "webhook_endpoint_verification_jobs", "INSERT", false},
		{"ledgersync_support_readonly", "audit_events", "SELECT", true},
		{"ledgersync_support_readonly", "investigation_saved_views", "SELECT", true},
		{"ledgersync_support_readonly", "investigation_workspaces", "SELECT", true},
		{"ledgersync_support_readonly", "investigation_workspace_references", "SELECT", true},
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

	var localLoginsExist bool
	if err := database.QueryRowContext(context.Background(), `
SELECT to_regrole('ledgersync_local_api') IS NOT NULL
   AND to_regrole('ledgersync_local_worker') IS NOT NULL`).Scan(&localLoginsExist); err != nil {
		t.Fatal(err)
	}
	if !localLoginsExist {
		return
	}
	loginChecks := []struct {
		login, expectedGroup, forbiddenGroup string
	}{
		{"ledgersync_local_api", "ledgersync_api", "ledgersync_worker"},
		{"ledgersync_local_worker", "ledgersync_worker", "ledgersync_api"},
	}
	for _, check := range loginChecks {
		var canLogin, superuser, createDB, createRole, replication, bypassRLS bool
		if err := database.QueryRowContext(context.Background(), `
SELECT rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolbypassrls
FROM pg_roles WHERE rolname=$1`, check.login).Scan(&canLogin, &superuser, &createDB, &createRole, &replication, &bypassRLS); err != nil {
			t.Fatal(err)
		}
		if !canLogin || superuser || createDB || createRole || replication || bypassRLS {
			t.Errorf("unsafe local login attributes for %s", check.login)
		}
		var intendedMemberships, otherMemberships int
		if err := database.QueryRowContext(context.Background(), `
SELECT count(*) FILTER (WHERE granted.rolname=$2),
       count(*) FILTER (WHERE granted.rolname<>$2)
FROM pg_auth_members memberships
JOIN pg_roles member ON member.oid=memberships.member
JOIN pg_roles granted ON granted.oid=memberships.roleid
WHERE member.rolname=$1`, check.login, check.expectedGroup).Scan(&intendedMemberships, &otherMemberships); err != nil {
			t.Fatal(err)
		}
		if intendedMemberships != 1 || otherMemberships != 0 {
			t.Errorf("%s memberships intended=%d other=%d", check.login, intendedMemberships, otherMemberships)
		}
		var siblingMember bool
		if err := database.QueryRowContext(context.Background(), `SELECT pg_has_role($1,$2,'MEMBER')`, check.login, check.forbiddenGroup).Scan(&siblingMember); err != nil {
			t.Fatal(err)
		}
		if siblingMember {
			t.Errorf("%s inherited sibling workload role %s", check.login, check.forbiddenGroup)
		}
	}
}
