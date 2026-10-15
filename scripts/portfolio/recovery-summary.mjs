// Only this bounded projection can enter the browser bundle. Backup bytes,
// paths, container identities, rows and login details remain private.
export function recoverySummary(manifest, restore, result) {
  const required = ['candidate-recovery-fixture', 'candidate-recovery-restored', 'candidate-recovery-migrations', 'source-and-target-preservation', 'disposable-cleanup'];
  if (manifest.purpose !== 'run-evidence' || !/^demo-\d{14}-[a-f0-9]{8}$/.test(manifest.run_id) || !/^[a-f0-9]{40}$/.test(manifest.source.commit) || !/^[a-f0-9]{64}$/.test(manifest.source.manifest_sha256)
    || manifest.scenarios.length !== required.length || required.some(id => manifest.scenarios.filter(s => s.id === id && s.status === 'passed').length !== 1)
    || result.status !== 'verified_local' || result.source_and_target_distinct !== true || result.managed_recovery_claim !== false
    || restore.source_and_target_distinct !== true || restore.rows_filtered !== false
    || ![restore.backup_seconds, restore.restore_setup_seconds, result.backup_and_validated_restore_seconds].every(v => Number.isFinite(v) && v > 0 && v <= 600)) throw new Error('Complete guarded candidate recovery evidence required');
  return {
    schema_version: 'ledgersync-candidate-recovery-summary/v1', run_id: manifest.run_id,
    completed_at: manifest.completed_at, source_commit: manifest.source.commit,
    source_manifest_sha256: manifest.source.manifest_sha256, migration_count: manifest.migration.count,
    status: 'verified_local', backup_seconds: restore.backup_seconds,
    restore_setup_seconds: restore.restore_setup_seconds,
    backup_and_validation_seconds: result.backup_and_validated_restore_seconds,
    checks: required, scope: 'Synthetic local candidate; separate databases; no normal-runtime coverage or managed recovery target.'
  };
}
