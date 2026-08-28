package contract_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecoveryComposeMountIsOneFixedReadOnlySanitizedIndex(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repositoryRoot(t), "deploy", "compose", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	compose := string(content)
	want := "../../data/local-backups/recovery-evidence-index.json:/run/ledgersync/recovery/recovery-evidence-index.json:ro"
	if strings.Count(compose, want) != 1 || !strings.Contains(compose, "LEDGERSYNC_RECOVERY_EVIDENCE_ROOT: /run/ledgersync/recovery") {
		t.Fatalf("API recovery single-file bind or configured root drifted")
	}
	for _, forbidden := range []string{
		"../../data/local-backups:/run/ledgersync/recovery",
		"../../data/local-backups:/run/ledgersync/backups",
		"/var/run/docker.sock",
	} {
		if strings.Contains(compose, forbidden) {
			t.Fatalf("Compose exposes forbidden recovery/infrastructure mount %q", forbidden)
		}
	}
}

func TestPhase7OpenAPIDeclaresBoundedReadOnlyEvidenceSurface(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repositoryRoot(t), "contracts", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	contract := string(content)
	for _, marker := range []string{
		"/recovery/manifests:", "/exports/transfers.csv:", "/exports/accounts/{accountId}/transactions.csv:", "/exports/reconciliation.csv:",
		"RecoveryEvidenceIndex:", "ledgersync-recovery-evidence-index/v1", "maximum: 10000", "contentMediaType: text/csv",
		"spreadsheet-active text neutralized", "X-LedgerSync-Export-Rows", "recovery:read", "exports:read",
	} {
		if !strings.Contains(contract, marker) {
			t.Errorf("Phase 7 OpenAPI missing %q", marker)
		}
	}
	for _, forbidden := range []string{"dump_filename:", "digest_value:", "database_url:", "docker_socket:", "requestBody:"} {
		if forbidden != "requestBody:" && strings.Contains(contract[strings.Index(contract, "/recovery/manifests:"):strings.Index(contract, "/developer/metadata:")], forbidden) {
			t.Errorf("Phase 7 OpenAPI exposes forbidden field %q", forbidden)
		}
	}
}
