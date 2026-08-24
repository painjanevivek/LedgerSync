package recovery

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestIndexReadsOnlyFixedContainedSanitizedV1File(t *testing.T) {
	root := t.TempDir()
	writeRecoveryIndex(t, root, validRecoveryIndex())
	index, err := NewManifestIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := index.Snapshot(context.Background())
	if err != nil || snapshot.LatestBackup == nil || snapshot.LatestRestore == nil {
		t.Fatalf("snapshot=%+v err=%v", snapshot, err)
	}
	if snapshot.LatestBackup.BackupID != "backup-20260825T010203Z-aaaaaaa" || snapshot.LatestBackup.SizeBytes != 8 || snapshot.LatestRestore.ReconciliationStatus != "matched" || snapshot.Retention.ValidBackupCount != 1 {
		t.Fatalf("snapshot drifted: %+v", snapshot)
	}
	encoded, _ := json.Marshal(snapshot)
	for _, forbidden := range []string{root, "database.dump", "sha256", "password", "secret", "token", "docker"} {
		if strings.Contains(strings.ToLower(string(encoded)), strings.ToLower(forbidden)) {
			t.Fatalf("snapshot exposed prohibited material %q: %s", forbidden, encoded)
		}
	}
}

func TestManifestIndexAcceptsInitializedEmptyEvidenceIndex(t *testing.T) {
	root := t.TempDir()
	writeRecoveryIndex(t, root, map[string]any{
		"format_version":   recoveryEvidenceFormat,
		"generated_at_utc": "2026-08-25T02:03:04Z",
		"latest_backup":    nil,
		"latest_restore":   nil,
		"retention":        map[string]any{"valid_backup_count": 0, "ignored_entry_count": 0, "configured_keep_count": 5},
	})
	index, _ := NewManifestIndex(root)
	snapshot, err := index.Snapshot(context.Background())
	if err != nil || snapshot.LatestBackup != nil || snapshot.LatestRestore != nil || snapshot.Retention.ValidBackupCount != 0 {
		t.Fatalf("empty initialized snapshot=%+v error=%v", snapshot, err)
	}
}

func TestManifestIndexRejectsMalformedUnknownIncompleteAndOversizedFiles(t *testing.T) {
	for name, content := range map[string][]byte{
		"malformed":  []byte(`{"format_version":`),
		"unknown":    mutateRecoveryIndex(t, "unexpected", "value"),
		"incomplete": []byte(`{"format_version":"ledgersync-recovery-evidence-index/v1"}`),
		"oversized":  []byte(strings.Repeat("x", maxRecoveryEvidenceBytes+1)),
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, recoveryEvidenceFileName), content, 0o600); err != nil {
				t.Fatal(err)
			}
			index, _ := NewManifestIndex(root)
			if _, err := index.Snapshot(context.Background()); err == nil {
				t.Fatal("hostile recovery evidence was accepted")
			}
		})
	}
}

func TestManifestIndexRejectsNonFinalOrUnsafeEvidenceValues(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any){
		"arbitrary backup id": func(value map[string]any) { value["latest_backup"].(map[string]any)["backup_id"] = "../../dump" },
		"unverified digest":   func(value map[string]any) { value["latest_backup"].(map[string]any)["digest_status"] = "mismatch" },
		"failed validation":   func(value map[string]any) { value["latest_backup"].(map[string]any)["validation_status"] = "failed" },
		"restore mismatch":    func(value map[string]any) { value["latest_restore"].(map[string]any)["mismatch_count"] = 1 },
		"changed project": func(value map[string]any) {
			value["latest_restore"].(map[string]any)["normal_project_unchanged"] = false
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			value := validRecoveryIndex()
			mutate(value)
			writeRecoveryIndex(t, root, value)
			index, _ := NewManifestIndex(root)
			if _, err := index.Snapshot(context.Background()); err == nil {
				t.Fatal("unsafe recovery state was accepted")
			}
		})
	}
}

func TestManifestIndexRejectsSymlinkOrReparsePointFile(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	writeRecoveryIndex(t, outside, validRecoveryIndex())
	if err := os.Symlink(filepath.Join(outside, recoveryEvidenceFileName), filepath.Join(root, recoveryEvidenceFileName)); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	index, _ := NewManifestIndex(root)
	if _, err := index.Snapshot(context.Background()); err == nil {
		t.Fatal("symlinked evidence index was accepted")
	}
}

func TestManifestIndexHonorsCancellation(t *testing.T) {
	root := t.TempDir()
	writeRecoveryIndex(t, root, validRecoveryIndex())
	index, _ := NewManifestIndex(root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := index.Snapshot(ctx); err == nil {
		t.Fatal("canceled recovery evidence read succeeded")
	}
}

func validRecoveryIndex() map[string]any {
	return map[string]any{
		"format_version":   recoveryEvidenceFormat,
		"generated_at_utc": "2026-08-25T02:03:04Z",
		"latest_backup": map[string]any{
			"backup_id": "backup-20260825T010203Z-aaaaaaa", "finalized_at_utc": "2026-08-25T01:02:03Z", "size_bytes": 8, "schema_version": "000015_operations_read_models.up.sql", "digest_status": "verified", "validation_status": "passed", "source_commit": strings.Repeat("a", 40),
		},
		"latest_restore": map[string]any{
			"backup_id": "backup-20260825T010203Z-aaaaaaa", "completed_at_utc": "2026-08-25T02:00:00Z", "status": "passed", "reconciliation_status": "matched", "mismatch_count": 0, "normal_project_unchanged": true, "local_rto_seconds": 4.25,
		},
		"retention": map[string]any{"valid_backup_count": 1, "ignored_entry_count": 2, "configured_keep_count": 5},
	}
}

func writeRecoveryIndex(t *testing.T, root string, value map[string]any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, recoveryEvidenceFileName), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

func mutateRecoveryIndex(t *testing.T, key string, value any) []byte {
	t.Helper()
	payload := validRecoveryIndex()
	payload[key] = value
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
