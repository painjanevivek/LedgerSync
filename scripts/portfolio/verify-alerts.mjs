import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repository = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const require = createRequire(import.meta.url);
const YAML = require('../../web/node_modules/yaml');
const rules = YAML.parse(readFileSync(resolve(repository, 'deploy/observability/alerts.yml'), 'utf8'));
for (const rule of rules.groups.flatMap(group => group.rules)) {
  const path = rule.annotations?.runbook;
  if (!/^docs\/runbooks\/[a-z0-9-]+\.md$/.test(path ?? '') || !existsSync(resolve(repository, path))) throw Error('Every alert needs an existing repository runbook');
}
const tool = process.env.LEDGERSYNC_PROMTOOL_PATH;
if (!tool || !existsSync(tool) || process.argv.length !== 2) throw Error('Set LEDGERSYNC_PROMTOOL_PATH to the verified native promtool 3.15.0 executable');
const run = args => execFileSync(tool, args, { cwd: repository, windowsHide: true, timeout: 30_000, maxBuffer: 1024 * 1024, encoding: 'utf8' });
if (!/version 3\.15\.0\b/.test(run(['--version']))) throw Error('Pinned native promtool 3.15.0 required');
run(['check', 'rules', 'deploy/observability/alerts.yml']);
run(['test', 'rules', 'deploy/observability/alerts.test.yml']);
console.log(JSON.stringify({ status: 'passed', evaluator: 'promtool 3.15.0', cases: 3, runbook_links: rules.groups.flatMap(group => group.rules).length, scope: 'native synthetic rule evaluation; no server or notifications started' }, null, 2));
