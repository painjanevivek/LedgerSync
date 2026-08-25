package contract_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestBaselineFinancialMigrationsRemainImmutable(t *testing.T) {
	expected := map[string]string{
		"000001_financial_schema.up.sql":                "238f61ad3bd39bba15e9bda2d3eb4a4bc43d69279be48939ae311859249aa27d",
		"000002_transfer_ledger.up.sql":                 "8175ccb071bf3a456cebdde271425c4b25d6e7f8d029407ac812fff950e626a6",
		"000003_ledger_integrity.up.sql":                "d133244f42644c9dbd4b1eb59adeacead85dc046d1ebe2a45b44f230bc3268ce",
		"000004_outbox_delivery_leases.up.sql":          "c41a7cf78b89989ee676998e850f90603432dbf782c277f4230e51515498470b",
		"000005_operational_evidence.up.sql":            "45136cb40fc7edcd304d437dfb080305dc7d2e869ce9bb94f3f615e1fbe5e309",
		"000006_reconciliation_opening_balances.up.sql": "6ef3da3080bb65a4a4ac2376cf670fa482cac5ee9b7c19bfc010099d935d8976",
		"000007_operator_investigation.up.sql":          "ac195eb1752f0380a9d3e89cf534092e239dfe08c706c5d32a5caffdbb5f88aa",
		"000008_reconciliation_delivery_truth.up.sql":   "9bbdd9ea20798767af11bc313b99c06025588a376557da818e4f8b307801c520",
		"000009_pilot_security_controls.up.sql":         "2929f0c8667879983f8d5d77b15201434174242b420a83618af4908ce0782a87",
		"000010_lifecycle_recovery_provisioning.up.sql": "e8dd87611fb2693f1028e23101b9d4a00f47d52b8d6c43c48c6cee366e1b53b0",
		"000011_account_directory_scale.up.sql":         "ac5ac9eac3c72c830907d52056af3b1071366c43f7d74696203d656aef2be78e",
		"000012_transfer_velocity_capacity.up.sql":      "7ece49263be7f9ef4ccb7cab17f39a5cfbd421c22f33a41cb00d495e2cbde5ae",
	}

	root := filepath.Join(repositoryRoot(t), "migrations")
	for name, want := range expected {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read frozen migration %s: %v", name, err)
		}
		// Git may materialize text files with CRLF on Windows. Hash canonical LF
		// bytes so the immutability contract protects SQL content identically on
		// Windows and Linux without rewriting a frozen migration.
		canonical := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
		if got := fmt.Sprintf("%x", sha256.Sum256(canonical)); got != want {
			t.Errorf("frozen migration %s changed: got %s want %s; add a forward-only migration instead", name, got, want)
		}
	}
}

func TestFinancialSourcePathsDoNotUseFloatingPointMoney(t *testing.T) {
	root := repositoryRoot(t)
	paths := []string{
		filepath.Join(root, "internal", "domain", "money"),
		filepath.Join(root, "internal", "domain", "ledger"),
		filepath.Join(root, "internal", "domain", "account"),
		filepath.Join(root, "internal", "application", "transfers"),
		filepath.Join(root, "internal", "platform", "db", "transfer_repository.go"),
	}
	forbidden := regexp.MustCompile(`\b(float32|float64|ParseFloat|FormatFloat)\b`)

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			assertNoFloatingPointMoney(t, path, forbidden)
			continue
		}
		err = filepath.WalkDir(path, func(candidate string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(candidate) != ".go" || filepath.Ext(candidate) == ".test" {
				return nil
			}
			assertNoFloatingPointMoney(t, candidate, forbidden)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func assertNoFloatingPointMoney(t *testing.T, path string, forbidden *regexp.Regexp) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if match := forbidden.Find(content); match != nil {
		t.Errorf("financial source %s contains forbidden floating-point operation %q", path, match)
	}
}
