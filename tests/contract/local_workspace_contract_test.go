package contract_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLocalWorkspaceBootstrapCreatesPolicyWithoutFinancialEvidence(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate local workspace contract test")
	}
	content, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "deploy", "compose", "local-bootstrap.sql"))
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ToLower(string(content))
	for _, required := range []string{
		"insert into tenants",
		"insert into tenant_subject_roles",
		"insert into tenant_funding_policies",
		"insert into tenant_transfer_policies",
		"local-user",
	} {
		if !strings.Contains(source, required) {
			t.Errorf("local bootstrap is missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"insert into accounts",
		"insert into account_balance_projections",
		"insert into transfers",
		"insert into journal_transactions",
		"insert into ledger_postings",
		"insert into reconciliation_runs",
	} {
		if strings.Contains(source, forbidden) {
			t.Errorf("local bootstrap creates financial evidence through %q", forbidden)
		}
	}
}
