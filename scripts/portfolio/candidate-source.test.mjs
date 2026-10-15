import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, copyFileSync, writeFileSync, readdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { repository, captureCandidateSource, verifyBuildContext } from './candidate-source.mjs';

test('candidate context rejects private inputs and altered reviewed source', () => {
  const root = mkdtempSync(join(tmpdir(), 'ledgersync-context-'));
  const migration = readdirSync(join(repository, 'migrations')).filter(name => name.endsWith('.up.sql')).sort()[0];
  for (const name of ['go.mod', 'web/package-lock.json', `migrations/${migration}`]) {
    mkdirSync(dirname(join(root, name)), { recursive: true }); copyFileSync(join(repository, name), join(root, name));
  }
  assert.equal(verifyBuildContext(root).status, 'verified_local');
  writeFileSync(join(root, 'go.mod'), 'altered source');
  assert.throws(() => verifyBuildContext(root), /differs from current source/);
  copyFileSync(join(repository, 'go.mod'), join(root, 'go.mod'));
  writeFileSync(join(root, 'web', '.env.private'), 'synthetic fixture');
  assert.throws(() => verifyBuildContext(root), /Private or unintended/);
  assert.throws(() => captureCandidateSource(join(tmpdir(), 'candidate-outside-workspace')), /Exact private/);
});
