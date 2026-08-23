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
	for _, role := range []string{"ledgersync_migration_owner", "ledgersync_api", "ledgersync_worker", "ledgersync_reconciliation", "ledgersync_support_readonly", "ledgersync_break_glass"} {
		if !strings.Contains(contract, "CREATE ROLE "+role+" NOLOGIN") {
			t.Errorf("missing NOLOGIN role %s", role)
		}
	}
	if strings.Contains(contract, "GRANT INSERT ON audit_events TO ledgersync_support_readonly") || strings.Contains(contract, "TO ledgersync_break_glass;") {
		t.Fatal("support or break-glass role received forbidden standing write authority")
	}
	for _, table := range []string{"api_rate_limit_windows", "journal_transactions", "ledger_postings", "delivery_attempts", "reconciliation_runs", "reconciliation_mismatches"} {
		if !strings.Contains(contract, table) {
			t.Errorf("API investigation/rate-limit contract omits %s", table)
		}
	}
}
