import test from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { resolve } from 'node:path';
import { readdirSync, readFileSync } from 'node:fs';
import { failureScenarios, requiredFailureMatrix, demoPreview } from './demo-scenarios.mjs';
import { canSaveFailureSummary } from './demo-safety.mjs';

test('all 13 required matrix rows map to fixed executable test names', () => {
  assert.equal(Object.keys(requiredFailureMatrix).length, 13);
  assert.equal(new Set(failureScenarios.map(row => row[0])).size, failureScenarios.length);
  for (const id of Object.values(requiredFailureMatrix)) assert.ok(failureScenarios.some(row => row[0] === id && /^Test[A-Za-z0-9]+$/.test(row[2])));
  assert.equal(demoPreview().execution, 'none');
  for (const [, group, name] of failureScenarios) {
    const source = readdirSync(`tests/${group}`).filter(file=>file.endsWith('.go')).map(file=>readFileSync(`tests/${group}/${file}`,'utf8')).join('\n');
    assert.ok(source.includes(`func ${name}(t *testing.T)`), `Missing executable scenario ${name}`);
  }
  const complete={scenarios:failureScenarios.map(([id])=>({id,status:'incomplete'}))};
  assert.equal(canSaveFailureSummary({profile:'failures',failure_matrix_version:'r07/v2'},complete),true);
  assert.equal(canSaveFailureSummary({profile:'failures',failure_matrix_version:'unknown'},complete),false);
  assert.equal(canSaveFailureSummary({profile:'failures',failure_matrix_version:'r07/v2'},{scenarios:complete.scenarios.slice(1)}),false);
});
test('preview succeeds with child execution and filesystem writes forbidden', () => {
  const result = execFileSync(process.execPath, ['--permission', `--allow-fs-read=${resolve('.')}`, 'scripts/portfolio/run-demo.mjs', '--preview'], { encoding: 'utf8', timeout: 10_000, windowsHide: true });
  const preview = JSON.parse(result);
  assert.equal(preview.creates_resources, false);
  assert.equal(preview.writes_files, false);
  assert.equal(preview.scenarios.length, failureScenarios.length);
});
