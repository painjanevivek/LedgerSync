import test from 'node:test';
import assert from 'node:assert/strict';
import { recoverySummary } from './recovery-summary.mjs';

test('recovery summary requires each real profile check and exposes only bounded evidence', () => {
  const manifest = { purpose: 'run-evidence', run_id: 'demo-20261015140457-e13b773e', source: { commit: 'a'.repeat(40), manifest_sha256: 'b'.repeat(64) }, migration: { count: 61 }, completed_at: '2026-10-15T14:06:00.000Z', scenarios: ['candidate-recovery-fixture', 'candidate-recovery-restored', 'candidate-recovery-migrations', 'source-and-target-preservation', 'disposable-cleanup'].map(id => ({ id, status: 'passed' })) };
  const restore = { source_and_target_distinct: true, rows_filtered: false, backup_seconds: 1, restore_setup_seconds: 5, password: 'must-not-leak', source_database_id: 'private' };
  const result = { status: 'verified_local', source_and_target_distinct: true, managed_recovery_claim: false, backup_and_validated_restore_seconds: 9 };
  const safe = recoverySummary(manifest, restore, result);
  assert.equal(safe.status, 'verified_local'); assert.ok(!JSON.stringify(safe).includes('private')); assert.ok(!JSON.stringify(safe).includes('must-not-leak'));
  for (const mutate of [m => m.scenarios.pop(), m => m.scenarios[0].status = 'failed', m => m.scenarios.push(m.scenarios[0]), m => m.source.manifest_sha256 = 'unbound', m => m.purpose = 'contract-fixture']) { const value = structuredClone(manifest); mutate(value); assert.throws(() => recoverySummary(value, restore, result)); }
  for (const mutate of [r => r.source_and_target_distinct = false, r => r.rows_filtered = true, r => r.backup_seconds = NaN]) { const value = structuredClone(restore); mutate(value); assert.throws(() => recoverySummary(manifest, value, result)); }
  assert.throws(() => recoverySummary(manifest, restore, { ...result, managed_recovery_claim: true }));
});
