import { readFileSync, writeFileSync, existsSync, mkdirSync, realpathSync } from 'node:fs';
import { resolve, dirname, basename } from 'node:path';
import { fileURLToPath } from 'node:url';
import { readDemoManifest } from './demo-contract.mjs';
import { bundlePath, verifyDemoArtifacts } from './demo-safety.mjs';
import { recoverySummary } from './recovery-summary.mjs';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
if (![3, 4].includes(process.argv.length) || process.argv.length === 4 && process.argv[3] !== '--save-summary') throw new Error('Usage: node scripts/portfolio/verify-recovery.mjs <run-manifest.json> [--save-summary]');
const path = resolve(process.argv[2]), root = dirname(path);
if (basename(path) !== 'run-manifest.json' || dirname(root) !== resolve(repo, 'data/portfolio/demo-runs') || !/^demo-\d{14}-[a-f0-9]{8}$/.test(basename(root)) || realpathSync(root) !== root) throw new Error('Exact local recovery bundle required');
const manifest = readDemoManifest(bundlePath(root, 'run-manifest.json'));
const verified = await verifyDemoArtifacts(manifest, root);
for (const name of ['candidate-restore.json', 'candidate-recovery-result.json', 'candidate-backup.json', 'database.dump', 'recovery-reference.json']) if (!manifest.artifacts.some(a => a.path === name)) throw new Error('Recovery artifact missing from manifest');
const summary = recoverySummary(manifest, JSON.parse(readFileSync(bundlePath(root, 'candidate-restore.json'), 'utf8')), JSON.parse(readFileSync(bundlePath(root, 'candidate-recovery-result.json'), 'utf8')));
if (process.argv[3] === '--save-summary') {
  const archive = resolve(repo, 'docs/release-evidence/portfolio/recovery'); mkdirSync(archive, { recursive: true });
  if (realpathSync(archive) !== archive) throw new Error('Unsafe recovery summary destination');
  const content = JSON.stringify(summary, null, 2) + '\n', saved = resolve(archive, `${summary.run_id}.json`);
  if (existsSync(saved) && readFileSync(saved, 'utf8') !== content) throw new Error('Historical summary differs');
  if (!existsSync(saved)) writeFileSync(saved, content, { flag: 'wx' });
  writeFileSync(resolve(repo, 'web/src/lib/portfolio-recovery.generated.json'), content);
}
console.log(JSON.stringify({ run_id: summary.run_id, ...verified, status: summary.status }));
