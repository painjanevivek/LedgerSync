import { createHash } from 'node:crypto';
import { createReadStream, lstatSync, realpathSync, statSync, readFileSync } from 'node:fs';
import { resolve, relative, isAbsolute, sep } from 'node:path';
import { validateDemoManifest } from './demo-contract.mjs';
import { failureScenarios } from './demo-scenarios.mjs';

export const demoPurpose = 'portfolio-failure-demo';
export function canSaveFailureSummary(profile,manifest){
  if(profile?.failure_matrix_version!==undefined&&profile.failure_matrix_version!=='r07/v2')return false;
  const expected=profile?.failure_matrix_version==='r07/v2'?failureScenarios.map(row=>row[0]):['lost-response','duplicate-command','duplicate-and-reordered-event','stale-worker','cache-loss','dependency-loss','process-crash'];
  const extras=['source-and-target-preservation','runner-completion','disposable-cleanup'];
  return profile?.profile==='failures'&&expected.every(id=>manifest.scenarios.filter(s=>s.id===id).length===1)&&manifest.scenarios.every(s=>expected.includes(s.id)||extras.includes(s.id));
}
export function assertDisposableTarget(value, expected, {allowStopped=false,allowPaused=false}={}) {
  if (!/^ledgersync-acceptance-\d{14}-[a-f0-9]{8}$/.test(expected.project) || !/^[a-f0-9]{64}$/.test(expected.id) || !/^sha256:[a-f0-9]{64}$/.test(expected.image)) throw new Error('Invalid disposable identity');
  if (value.Id !== expected.id || value.Image !== expected.image || value.Config?.Labels?.['com.docker.compose.project'] !== expected.project || value.Config?.Labels?.['ledgersync.purpose'] !== demoPurpose) throw new Error('Disposable target identity mismatch');
  const path = expected.service === 'postgres' ? '/var/lib/postgresql/data' : expected.service === 'redis' ? '/data' : null;
  const size = expected.service === 'postgres' ? 'rw,size=512m' : 'rw,size=128m';
  const temporary = value.HostConfig?.Tmpfs ?? {};
  // Docker can expose tmpfs in HostConfig without a Mounts entry.
  if ((!allowStopped&&!value.State?.Running)||(!allowPaused&&value.State?.Paused) || !path || Object.keys(temporary).length !== 1 || temporary[path] !== size || !Array.isArray(value.Mounts) || value.Mounts.some(m => m.Type !== 'tmpfs')) throw new Error('Disposable target requires running temporary storage only');
  let bindings = Object.values(value.NetworkSettings?.Ports ?? {}).filter(Boolean).flat();
  if(bindings.length===0&&((allowPaused&&value.State?.Paused)||(allowStopped&&!value.State?.Running))){
    // Docker Desktop can omit assigned dynamic ports while paused or stopped.
    // Only an identity already recorded by this run can use this cleanup path.
    const declared=value.HostConfig?.PortBindings??{},key=expected.service==='postgres'?'5432/tcp':'6379/tcp',entries=declared[key];
    if(!Number.isInteger(expected.port)||expected.port<1||expected.port>65535||Object.keys(declared).length!==1||!Array.isArray(entries)||entries.length!==1||entries[0].HostIp!=='127.0.0.1'||![String(expected.port),''].includes(entries[0].HostPort))throw new Error('Paused or stopped target has no verified loopback allocation');
    bindings=[{HostIp:'127.0.0.1',HostPort:String(expected.port)}];
  }
  if (bindings.length !== 1 || bindings.some(b => b.HostIp !== '127.0.0.1' || !/^[1-9][0-9]{0,4}$/.test(b.HostPort) || Number(b.HostPort) > 65535)) throw new Error('Disposable target must bind one loopback port');
  if(expected.port!==undefined&&Number(bindings[0].HostPort)!==expected.port)throw new Error('Disposable target allocated port changed');
  return Number(bindings[0].HostPort);
}

export function bundlePath(root, name) {
  if (!/^[a-zA-Z0-9_][a-zA-Z0-9_./-]*$/.test(name) || name.split('/').some(p => !p || p === '.' || p === '..')) throw new Error('Unsafe artifact path');
  const absolute = resolve(root, name), offset = relative(realpathSync(root), absolute);
  if (!offset || isAbsolute(offset) || offset.startsWith('..' + sep) || offset === '..') throw new Error('Artifact escaped bundle');
  let cursor = resolve(root);
  for (const part of name.split('/')) { cursor = resolve(cursor, part); if (lstatSync(cursor).isSymbolicLink()) throw new Error('Artifact symlinks are forbidden'); }
  if (realpathSync(absolute) !== absolute || !statSync(absolute).isFile()) throw new Error('Artifact is not a regular bundle file');
  return absolute;
}

export async function fileDigest(path) {
  const digest = createHash('sha256'); for await (const chunk of createReadStream(path)) digest.update(chunk); return digest.digest('hex');
}

export function scenarioResult(lines, expectedTest, exitCode) {
  if (lines.length > 8 * 1024 * 1024) throw new Error('Scenario output exceeds bound');
  const records = lines.trim().split(/\r?\n/).filter(Boolean).map(line => JSON.parse(line));
  const pass = records.filter(r => r.Action === 'pass' && r.Test === expectedTest);
  if (exitCode !== 0 || pass.length !== 1 || records.some(r => ['fail', 'skip'].includes(r.Action))) return { status: 'failed', reason: 'The exact named test did not complete without failure or skip.', checks: [{ id: 'named-runtime-test', status: 'failed', reason: 'Inspect the bounded scenario log.' }] };
  return { status: 'passed', checks: [{ id: 'named-runtime-test', status: 'passed' }] };
}

export async function verifyDemoArtifacts(manifest, root) {
  const validation = validateDemoManifest(manifest); if (!validation.valid) throw new Error('Manifest contract failed: ' + validation.errors.join('; '));
  if (manifest.purpose !== 'run-evidence' || /^0+$/.test(manifest.source.commit) || manifest.images.some(image => /^sha256:0+$/.test(image.digest))) throw new Error('Runtime provenance must be real, not a contract fixture');
  let total = 0;
  for (const artifact of manifest.artifacts) {
    const path = bundlePath(root, artifact.path), size = statSync(path).size;
    total += size; if (size !== artifact.bytes || total > 256 * 1024 * 1024 || await fileDigest(path) !== artifact.sha256) throw new Error('Artifact integrity failed: ' + artifact.path);
  }
  for (const [name, digest] of [['source-manifest.json', manifest.source.manifest_sha256], ['migration-manifest.json', manifest.migration.manifest_sha256]]) {
    const artifact = manifest.artifacts.find(a => a.path === name);
    if (!artifact || artifact.sha256 !== digest) throw new Error('Missing bound provenance artifact: ' + name);
  }
  if(statSync(bundlePath(root,'source-manifest.json')).size>4*1024*1024||statSync(bundlePath(root,'migration-manifest.json')).size>1024*1024)throw new Error('Provenance document exceeds bound');
  const source = JSON.parse(readFileSync(bundlePath(root,'source-manifest.json'),'utf8'));
  if(source.head!==manifest.source.commit||source.dirty!==manifest.source.dirty||!Array.isArray(source.files)||source.files.length<1||source.files.length>20000||new Set(source.files.map(f=>f.path)).size!==source.files.length||source.files.some(f=>typeof f.path!=='string'||isAbsolute(f.path)||f.path.includes('\\')||f.path.split('/').some(p=>!p||p==='.'||p==='..')||! /^[a-f0-9]{64}$/.test(f.sha256)))throw new Error('Source provenance does not match manifest');
  const migration=JSON.parse(readFileSync(bundlePath(root,'migration-manifest.json'),'utf8'));
  if(!Array.isArray(migration.applied)||migration.applied.length!==manifest.migration.count||(migration.applied.at(-1)??'unavailable')!==manifest.migration.latest)throw new Error('Applied migration provenance does not match manifest');
  if(!Array.isArray(migration.source_migrations)||migration.applied.some(version=>!migration.source_migrations.some(file=>file.path===version&&/^[a-f0-9]{64}$/.test(file.sha256))))throw new Error('Applied migration source identity missing');
  return { verified: true, artifacts: manifest.artifacts.length, bytes: total };
}

export function publicDemoSummary(manifest, verification) {
  if (!verification?.verified) throw new Error('Artifact verification is required before summary creation');
  return { schema_version: 'ledgersync-demo-summary/v1', run_id: manifest.run_id, completed_at: manifest.completed_at, purpose: 'historical-local-synthetic-demonstration', passed: manifest.scenarios.every(s=>s.status==='passed'), source: manifest.source, api_contract_version: manifest.api_contract_version, migration: { latest: manifest.migration.latest, count: manifest.migration.count, manifest_sha256: manifest.migration.manifest_sha256 }, images: manifest.images, scenarios: manifest.scenarios.map(s=>({id:s.id,status:s.status,...(s.reason?{reason:s.reason}:{})})), artifact_count: verification.artifacts, limitations: manifest.limitations };
}
