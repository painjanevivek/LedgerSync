export type RecoveryBackupEvidence = Readonly<{
  backup_id: string;
  finalized_at_utc: string;
  size_bytes: number;
  schema_version: string;
  digest_status: "verified";
  validation_status: "passed";
  source_commit: string;
}>;

export type RecoveryRestoreEvidence = Readonly<{
  backup_id: string;
  completed_at_utc: string;
  status: "passed";
  reconciliation_status: "matched" | "completed" | "passed";
  mismatch_count: 0;
  normal_project_unchanged: true;
  local_rto_seconds: number;
}>;

export type RecoveryEvidenceIndex = Readonly<{
  format_version: "ledgersync-recovery-evidence-index/v1";
  generated_at_utc: string;
  latest_backup: RecoveryBackupEvidence | null;
  latest_restore: RecoveryRestoreEvidence | null;
  retention: Readonly<{ valid_backup_count: number; ignored_entry_count: number; configured_keep_count: number }>;
}>;

export type SanitizedRecoveryResponse = Readonly<{ status: number; body: Readonly<Record<string, unknown>> }>;

const exactKeys = (value: Record<string, unknown>, allowed: readonly string[]) => Object.keys(value).every((key) => allowed.includes(key));
const record = (value: unknown): Record<string, unknown> | null => value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : null;
const dateTime = (value: unknown) => typeof value === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(value) && !Number.isNaN(Date.parse(value));
const backupID = (value: unknown) => typeof value === "string" && /^backup-\d{8}T\d{6}Z-[0-9a-f]{7,40}$/.test(value);
const boundedInteger = (value: unknown, minimum: number, maximum: number) => typeof value === "number" && Number.isSafeInteger(value) && value >= minimum && value <= maximum;

function backup(value: unknown): RecoveryBackupEvidence | null {
  const item = record(value);
  if (!item || !exactKeys(item, ["backup_id", "finalized_at_utc", "size_bytes", "schema_version", "digest_status", "validation_status", "source_commit"])) return null;
  if (!backupID(item.backup_id) || !dateTime(item.finalized_at_utc) || !boundedInteger(item.size_bytes, 1, Number.MAX_SAFE_INTEGER) || typeof item.schema_version !== "string" || !/^[0-9]{6}_[a-z0-9._-]{1,120}$/.test(item.schema_version) || item.digest_status !== "verified" || item.validation_status !== "passed" || typeof item.source_commit !== "string" || !/^[0-9a-f]{40}$/.test(item.source_commit)) return null;
  return item as RecoveryBackupEvidence;
}

function restore(value: unknown): RecoveryRestoreEvidence | null {
  const item = record(value);
  if (!item || !exactKeys(item, ["backup_id", "completed_at_utc", "status", "reconciliation_status", "mismatch_count", "normal_project_unchanged", "local_rto_seconds"])) return null;
  if (!backupID(item.backup_id) || !dateTime(item.completed_at_utc) || item.status !== "passed" || !["matched", "completed", "passed"].includes(String(item.reconciliation_status)) || item.mismatch_count !== 0 || item.normal_project_unchanged !== true || typeof item.local_rto_seconds !== "number" || !Number.isFinite(item.local_rto_seconds) || item.local_rto_seconds < 0 || item.local_rto_seconds > 86_400) return null;
  return item as RecoveryRestoreEvidence;
}

const publicErrors = new Set(["unauthorized", "forbidden", "rate_limited", "validation_failed", "recovery_evidence_unavailable", "upstream_timeout", "temporary_unavailable"]);

export function sanitizeRecoveryIndex(status: number, value: unknown): SanitizedRecoveryResponse {
  if (status >= 400) {
    const root = record(value); const error = record(root?.error); const code = typeof error?.code === "string" && publicErrors.has(error.code) ? error.code : "recovery_evidence_unavailable";
    return { status: [400, 401, 403, 429, 503, 504].includes(status) ? status : 503, body: { error: { code } } };
  }
  const root = record(value); const retention = record(root?.retention);
  if (!root || !exactKeys(root, ["format_version", "generated_at_utc", "latest_backup", "latest_restore", "retention"]) || root.format_version !== "ledgersync-recovery-evidence-index/v1" || !dateTime(root.generated_at_utc) || !retention || !exactKeys(retention, ["valid_backup_count", "ignored_entry_count", "configured_keep_count"]) || !boundedInteger(retention.valid_backup_count, 0, 200) || !boundedInteger(retention.ignored_entry_count, 0, 200) || !boundedInteger(retention.configured_keep_count, 1, 100)) return { status: 503, body: { error: { code: "recovery_evidence_unavailable" } } };
  const latestBackup = root.latest_backup === null ? null : backup(root.latest_backup);
  const latestRestore = root.latest_restore === null ? null : restore(root.latest_restore);
  if ((root.latest_backup !== null && !latestBackup) || (root.latest_restore !== null && !latestRestore) || (latestRestore && !latestBackup)) return { status: 503, body: { error: { code: "recovery_evidence_unavailable" } } };
  return { status: 200, body: { format_version: root.format_version, generated_at_utc: root.generated_at_utc, latest_backup: latestBackup, latest_restore: latestRestore, retention } };
}
