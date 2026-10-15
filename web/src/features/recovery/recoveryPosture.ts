import type { LocalDiagnostics } from "@/lib/api/operations";
import type { RecoveryEvidenceIndex } from "@/lib/api/recovery";

type Posture = Readonly<{ title: string; message: string; attention: boolean; command: "status" | "backup" | "restore" }>;

/** A valid historical backup is evidence for that snapshot, never current ledger coverage. */
export function recoveryPosture(diagnostics: LocalDiagnostics | null, recovery: RecoveryEvidenceIndex | null, current: boolean): Posture {
  if (!current) return { title: "Recovery evidence is not current", message: "Refresh the evidence when access and connectivity return. Retained results describe an earlier read.", attention: true, command: "status" };
  if (diagnostics?.financial_authority.postgres.state !== "reachable") return { title: "Recovery evidence needs one more proof", message: "Current PostgreSQL authority is not verified. Check local status before relying on backup evidence.", attention: true, command: "status" };
  const backup = recovery?.latest_backup;
  const restore = recovery?.latest_restore;
  if (!backup) return { title: "Recovery evidence needs one more proof", message: "No finalized backup is indexed. Create one with the protected host command.", attention: true, command: "backup" };
  const schema = diagnostics.financial_authority.postgres.schema_version;
  if (!schema) return { title: "Backup coverage is not verified", message: "The current schema is unavailable. Inspect local status before comparing it with the backup.", attention: true, command: "status" };
  if (backup.schema_version !== schema) return { title: "Recovery proof is historical", message: "The backup schema differs from the current database. Create a current backup, then run an isolated restore drill. The listed proof remains valid only for its recorded snapshot.", attention: true, command: "backup" };
  if (!restore || restore.backup_id !== backup.backup_id || Date.parse(restore.completed_at_utc) < Date.parse(backup.finalized_at_utc)) return { title: "Latest backup needs a restore drill", message: "No later passing drill names this exact backup. Run the isolated restore drill before relying on it.", attention: true, command: "restore" };
  return { title: "Listed backup restore verified", message: "The listed backup passed an isolated restore drill. This does not prove coverage of later writes or a production recovery target. Check the backup time against the records you need.", attention: false, command: "status" };
}
