import { readFileSync, statSync } from 'node:fs';
import { createRequire } from 'node:module';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

// Use the repository's locked web toolchain. This validator runs at build time.
const require = createRequire(new URL('../../web/package.json', import.meta.url));
const Ajv = require('ajv');
const schema = JSON.parse(readFileSync(new URL('../../contracts/demo/run-manifest.v1.schema.json', import.meta.url), 'utf8'));
const validateSchema = new Ajv({ allErrors: true, jsonPointers: true }).compile(schema);
export const maximumManifestBytes = 1024 * 1024;

export function validateDemoManifest(value) {
  if (!validateSchema(value)) return { valid: false, errors: validateSchema.errors.map(e => `${e.dataPath || '/'} ${e.message}`) };
  const errors = [];
  if (Date.parse(value.completed_at) < Date.parse(value.started_at)) errors.push('Completion precedes start.');
  const unique = (values, name) => {
    if (new Set(values).size !== values.length) errors.push(`Duplicate ${name}.`);
  };
  unique(value.scenarios.map(s => s.id), 'scenario id');
  unique(value.artifacts.map(a => a.path), 'artifact path');
  unique(value.images.map(i => i.service), 'image service');
  for (const scenario of value.scenarios) {
    unique(scenario.checks.map(c => c.id), `check id in ${scenario.id}`);
    if (scenario.status === 'passed' && (!scenario.checks.some(c => c.status === 'passed') || scenario.checks.some(c => c.status === 'failed'))) {
      errors.push(`Passed scenario ${scenario.id} lacks passing checks or contains a failed check.`);
    }
  }
  if (value.purpose === 'contract-fixture' && value.scenarios.some(s => s.status === 'passed')) errors.push('A synthetic fixture cannot claim a passed scenario.');
  if (value.purpose === 'run-evidence' && value.scenarios.some(s => s.status === 'passed') && value.artifacts.length === 0) errors.push('Passing run evidence requires artifacts.');
  for (const artifact of value.artifacts) {
    if (artifact.path.split('/').some(part => part === '.' || part === '..' || part === '')) errors.push('Artifact path contains an unsafe segment.');
  }
  return { valid: errors.length === 0, errors };
}

export function readDemoManifest(filename) {
  if (statSync(filename).size > maximumManifestBytes) throw new Error('Manifest exceeds the 1 MiB input limit.');
  const bytes = readFileSync(filename);
  if (bytes.length > maximumManifestBytes) throw new Error('Manifest exceeds the 1 MiB input limit.');
  return JSON.parse(bytes.toString('utf8'));
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    if (process.argv.length !== 3) throw new Error('Usage: node scripts/portfolio/demo-contract.mjs <manifest.json>');
    const result = validateDemoManifest(readDemoManifest(process.argv[2]));
    console.log(JSON.stringify(result));
    process.exitCode = result.valid ? 0 : 1;
  } catch (error) {
    console.error(error instanceof SyntaxError ? 'Manifest is not valid JSON.' : error.message);
    process.exitCode = 1;
  }
}
