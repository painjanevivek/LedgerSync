package contract_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDatabaseRoleContractKeepsSupportReadOnlyAndBreakGlassGrantless(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	content, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "deploy", "postgres", "roles.sql"))
	if err != nil {
		t.Fatal(err)
	}
	contract := string(content)
	for _, role := range []string{"ledgersync_migration_owner", "ledgersync_api", "ledgersync_worker", "ledgersync_reconciliation", "ledgersync_provisioning", "ledgersync_support_readonly", "ledgersync_break_glass"} {
		if !strings.Contains(contract, "CREATE ROLE "+role+" NOLOGIN") {
			t.Errorf("missing NOLOGIN role %s", role)
		}
	}
	if strings.Contains(contract, "GRANT INSERT ON audit_events TO ledgersync_support_readonly") || strings.Contains(contract, "TO ledgersync_break_glass;") {
		t.Fatal("support or break-glass role received forbidden standing write authority")
	}
	for _, table := range []string{"api_rate_limit_windows", "journal_transactions", "ledger_postings", "delivery_attempts", "delivery_replay_actions", "reconciliation_runs", "reconciliation_mismatches", "partner_credential_events"} {
		if !strings.Contains(contract, table) {
			t.Errorf("API investigation/rate-limit contract omits %s", table)
		}
	}
}

func TestLocalComposeUsesSeparateLeastPrivilegeWorkloadLogins(t *testing.T) {
	root := repositoryRoot(t)
	compose := readContractFile(t, filepath.Join(root, "deploy", "compose", "docker-compose.yml"))
	logins := readContractFile(t, filepath.Join(root, "deploy", "postgres", "local-workload-logins.sql"))
	dockerfile := readContractFile(t, filepath.Join(root, "deploy", "docker", "api.Dockerfile"))
	runtime := readContractFile(t, filepath.Join(root, "scripts", "local-runtime-common.ps1"))

	for _, marker := range []string{
		"postgres://ledgersync_local_api:${LEDGERSYNC_API_DATABASE_PASSWORD:",
		"postgres://ledgersync_local_worker:${LEDGERSYNC_WORKER_DATABASE_PASSWORD:",
		"target: database-tooling",
		"user: postgres",
		"/usr/local/bin/migrate &&",
		"-f /database-roles/roles.sql -f /database-roles/local-workload-logins.sql",
		"migrate:\n        condition: service_completed_successfully",
	} {
		if !strings.Contains(compose, marker) {
			t.Errorf("local Compose least-privilege boundary is missing %q", marker)
		}
	}
	if strings.Count(compose, "LEDGERSYNC_DATABASE_URL: postgres://ledgersync:${POSTGRES_PASSWORD:") != 1 {
		t.Fatal("database-owner DSN must be confined to the migration/role-provisioning job")
	}
	if strings.Index(compose, "/usr/local/bin/migrate &&") > strings.Index(compose, "-f /database-roles/roles.sql") {
		t.Fatal("database grants are not applied after migrations")
	}
	for _, marker := range []string{
		"\\set ON_ERROR_STOP on",
		"SELECT 1 / 0 AS invalid_workload_credentials;",
		"ALTER ROLE ledgersync_local_api WITH LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS",
		"ALTER ROLE ledgersync_local_worker WITH LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS",
		"GRANT ledgersync_api TO ledgersync_local_api;",
		"GRANT ledgersync_worker TO ledgersync_local_worker;",
		"REVOKE %I FROM %I",
	} {
		if !strings.Contains(logins, marker) {
			t.Errorf("local workload-login provisioning is missing %q", marker)
		}
	}
	for _, marker := range []string{
		"AS database-tooling",
		"COPY deploy/postgres /database-roles",
		"USER postgres",
		"LEDGERSYNC_API_DATABASE_PASSWORD",
		"LEDGERSYNC_WORKER_DATABASE_PASSWORD",
	} {
		if !strings.Contains(dockerfile+runtime, marker) {
			t.Errorf("local setup image/runtime credential contract is missing %q", marker)
		}
	}
}
