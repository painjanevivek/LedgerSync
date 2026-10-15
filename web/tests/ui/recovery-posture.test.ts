import assert from "node:assert/strict";
import test from "node:test";

import { recoveryPosture } from "../../src/features/recovery/recoveryPosture";
import type { LocalDiagnostics } from "../../src/lib/api/operations";
import type { RecoveryEvidenceIndex } from "../../src/lib/api/recovery";

const diagnostics: LocalDiagnostics = {
  overall_state: "ready", generated_at: "2026-10-10T12:10:00Z",
  application: { version: "test", commit: "test", environment: "local" },
  financial_authority: { postgres: { state: "reachable", schema_version: "000039_current.up.sql" }, latest_reconciliation: { state: "none" } },
  delivery_cache: { outbox: { state: "reachable", worker_progress: "idle" }, redis: { state: "reachable", label: "disposable_cache" } },
};
const backup = { backup_id: "backup-20261010T120000Z-abcdef1", finalized_at_utc: "2026-10-10T12:00:00Z", size_bytes: 100, schema_version: "000039_current.up.sql", digest_status: "verified" as const, validation_status: "passed" as const, source_commit: "a".repeat(40) };
const restore = { backup_id: backup.backup_id, completed_at_utc: "2026-10-10T12:01:00Z", status: "passed" as const, reconciliation_status: "matched" as const, mismatch_count: 0 as const, normal_project_unchanged: true as const, local_rto_seconds: 30 };
const recovery: RecoveryEvidenceIndex = { format_version: "ledgersync-recovery-evidence-index/v1", generated_at_utc: diagnostics.generated_at, latest_backup: backup, latest_restore: restore, retention: { valid_backup_count: 1, ignored_entry_count: 0, configured_keep_count: 5 } };

test("recovery proof cannot be current while offline, loading, or a refresh failed", () => {
  const posture = recoveryPosture(diagnostics, recovery, false);
  assert.equal(posture.attention, true);
  assert.equal(posture.command, "status");
  assert.equal(posture.title, "Recovery evidence is not current");
});

test("recovery requires current database and schema evidence", () => {
  assert.equal(recoveryPosture(null, recovery, true).command, "status");
  const noSchema = { ...diagnostics, financial_authority: { ...diagnostics.financial_authority, postgres: { state: "reachable" as const } } };
  assert.equal(recoveryPosture(noSchema, recovery, true).title, "Backup coverage is not verified");
});

test("recovery recommends a backup for absent or historical-schema proof", () => {
  assert.equal(recoveryPosture(diagnostics, { ...recovery, latest_backup: null, latest_restore: null }, true).command, "backup");
  assert.equal(recoveryPosture(diagnostics, { ...recovery, latest_backup: { ...backup, schema_version: "000016_previous.up.sql" } }, true).title, "Recovery proof is historical");
});

test("recovery rejects missing, wrong-backup, and earlier restore receipts", () => {
  for (const receipt of [null, { ...restore, backup_id: "backup-20261009T120000Z-abcdef1" }, { ...restore, completed_at_utc: "2026-10-10T11:59:59Z" }]) {
    const posture = recoveryPosture(diagnostics, { ...recovery, latest_restore: receipt }, true);
    assert.equal(posture.command, "restore");
    assert.equal(posture.attention, true);
  }
});

test("a matching restored backup has a bounded snapshot claim", () => {
  const posture = recoveryPosture(diagnostics, recovery, true);
  assert.equal(posture.title, "Listed backup restore verified");
  assert.equal(posture.attention, false);
  assert.match(posture.message, /does not prove coverage of later writes/);
});
