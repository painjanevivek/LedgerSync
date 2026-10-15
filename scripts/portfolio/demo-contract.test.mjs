import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { validateDemoManifest, readDemoManifest, maximumManifestBytes } from './demo-contract.mjs';

const fixture = JSON.parse(readFileSync(new URL('../../contracts/demo/run-manifest.v1.example.json', import.meta.url), 'utf8'));
test('the documented synthetic fixture is valid without claiming execution', () => {
  assert.deepEqual(validateDemoManifest(fixture), { valid: true, errors: [] });
});
const invalid = [
  ['unsupported version', v => { v.schema_version = 'ledgersync-demo-run/v2'; }],
  ['unrecognised field', v => { v.secret = 'must not enter the artifact'; }],
  ['nested unrecognised field', v => { v.images[0].password = 'not allowed'; }],
  ['normal runtime target', v => { v.isolated_project = 'compose'; }],
  ['invalid image digest', v => { v.images[0].digest = 'latest'; }],
  ['unbounded duration', v => { v.limits.duration_seconds = 0; }],
  ['reversed dates', v => { v.completed_at = '2026-01-01T00:00:00Z'; }],
  ['invalid calendar date', v => { v.started_at = 'not-a-date'; }],
  ['silent skipped result', v => { delete v.scenarios[0].reason; }],
  ['duplicate scenario', v => { v.scenarios.push(structuredClone(v.scenarios[0])); }],
  ['duplicate image service', v => { v.images.push(structuredClone(v.images[0])); }],
  ['fixture passed claim', v => { v.scenarios[0].status = 'passed'; v.scenarios[0].checks = [{ id: 'conservation', status: 'passed' }]; }],
  ['pass without checks', v => { v.purpose = 'run-evidence'; v.scenarios[0].status = 'passed'; }],
  ['pass with failed check', v => { v.purpose = 'run-evidence'; v.scenarios[0].status = 'passed'; v.scenarios[0].checks = [{ id: 'conservation', status: 'failed', reason: 'mismatch' }]; }],
  ['pass without artifacts', v => { v.purpose = 'run-evidence'; v.scenarios[0].status = 'passed'; v.scenarios[0].checks = [{ id: 'conservation', status: 'passed' }]; }],
];
for (const [name, mutate] of invalid) test(`reject ${name}`, () => {
  const value = structuredClone(fixture); mutate(value);
  assert.equal(validateDemoManifest(value).valid, false);
});
for (const path of ['../run.json', 'evidence/../run.json', '/run.json', 'C:/run.json', 'evidence\\run.json', 'evidence//run.json', 'evidence/./run.json']) test(`reject unsafe path ${path}`, () => {
  const value = structuredClone(fixture);
  value.artifacts = [{ path, sha256: 'a'.repeat(64), bytes: 1 }];
  assert.equal(validateDemoManifest(value).valid, false);
});
test('a run with a passing check and recorded artifact is structurally valid', () => {
  const value = structuredClone(fixture);
  value.purpose = 'run-evidence'; value.scenarios[0].status = 'passed';
  value.scenarios[0].checks = [{ id: 'conservation', status: 'passed' }];
  value.artifacts = [{ path: 'checks/conservation.json', sha256: 'a'.repeat(64), bytes: 42 }];
  assert.equal(validateDemoManifest(value).valid, true);
});
test('reject duplicate artifact and check identifiers', () => {
  const value = structuredClone(fixture);
  value.artifacts = Array(2).fill({ path: 'checks/a.json', sha256: 'a'.repeat(64), bytes: 1 });
  value.scenarios[0].checks = Array(2).fill({ id: 'same', status: 'passed' });
  assert.equal(validateDemoManifest(value).errors.length, 2);
});
test('bound file input and reject malformed JSON', () => {
  const folder = mkdtempSync(join(tmpdir(), 'ledgersync-contract-'));
  try {
    const file = join(folder, 'input.json');
    writeFileSync(file, ' '.repeat(maximumManifestBytes + 1));
    assert.throws(() => readDemoManifest(file), /1 MiB/);
    writeFileSync(file, '{');
    assert.throws(() => readDemoManifest(file), SyntaxError);
  } finally { rmSync(folder, { recursive: true, force: true }); }
});
