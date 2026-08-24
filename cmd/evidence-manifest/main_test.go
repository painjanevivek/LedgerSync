package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildManifestUsesCanonicalContractAndSortedMigrations(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "contracts", "openapi.yaml"), []byte("openapi: 3.1.0\n"))
	mustWrite(t, filepath.Join(root, "migrations", "0002_second.up.sql"), []byte("SELECT 2;"))
	mustWrite(t, filepath.Join(root, "migrations", "0001_first.up.sql"), []byte("SELECT 1;"))
	generatedAt := time.Date(2026, time.August, 24, 0, 0, 0, 0, time.FixedZone("test", 5*60*60+30*60))

	got, err := buildManifest(root, "test-commit", generatedAt)
	if err != nil {
		t.Fatal(err)
	}
	wantDigest := sha256.Sum256([]byte("openapi: 3.1.0\n"))
	if got.OpenAPISHA256 != hex.EncodeToString(wantDigest[:]) {
		t.Fatalf("OpenAPI digest = %q", got.OpenAPISHA256)
	}
	if want := []string{"0001_first.up.sql", "0002_second.up.sql"}; !reflect.DeepEqual(got.Migrations, want) {
		t.Fatalf("migrations = %v, want %v", got.Migrations, want)
	}
	if got.Commit != "test-commit" || got.GeneratedAt != "2026-08-23T18:30:00Z" {
		t.Fatalf("commit-bound metadata = %+v", got)
	}
}

func TestBuildManifestRejectsMissingCanonicalContract(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "migrations", "0001_first.up.sql"), []byte("SELECT 1;"))

	_, err := buildManifest(root, "test-commit", time.Time{})
	if err == nil || !strings.Contains(err.Error(), filepath.Join("contracts", "openapi.yaml")) {
		t.Fatalf("missing contract error = %v", err)
	}
}

func mustWrite(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}
