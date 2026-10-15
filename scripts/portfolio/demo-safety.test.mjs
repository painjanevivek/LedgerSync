import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync, rmSync, symlinkSync, mkdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname, basename } from 'node:path';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { assertDisposableTarget, bundlePath, demoPurpose, scenarioResult, verifyDemoArtifacts, canSaveFailureSummary } from './demo-safety.mjs';

const expected = { project: 'ledgersync-acceptance-20261015000000-12345678', id: 'a'.repeat(64), image: 'sha256:' + 'b'.repeat(64), service: 'postgres' };
const target = { Id: expected.id, Image: expected.image, Config: { Labels: { 'com.docker.compose.project': expected.project, 'ledgersync.purpose': demoPurpose } }, State: { Running: true }, Mounts: [], HostConfig: { Tmpfs: { '/var/lib/postgresql/data': 'rw,size=512m' } }, NetworkSettings: { Ports: { '5432/tcp': [{ HostIp: '127.0.0.1', HostPort: '54321' }] } } };
test('only the fixed failure profile updates its summary, including honest failed runs',()=>{
 const ids=['lost-response','duplicate-command','duplicate-and-reordered-event','stale-worker','cache-loss','dependency-loss','process-crash'];
 const manifest={scenarios:ids.map(id=>({id,status:'incomplete'}))};assert.equal(canSaveFailureSummary({profile:'failures'},manifest),true);
 manifest.scenarios.push({id:'runner-completion',status:'failed'});assert.equal(canSaveFailureSummary({profile:'failures'},manifest),true);
 for(const profile of ['capacity','financial-model','unknown'])assert.equal(canSaveFailureSummary({profile},manifest),false);
 assert.equal(canSaveFailureSummary({profile:'failures'},{scenarios:manifest.scenarios.slice(1)}),false);
 assert.equal(canSaveFailureSummary({profile:'failures'},{scenarios:[...manifest.scenarios,{id:'arbitrary-host-action'}]}),false);
});
test('browser summary is the exact safe projection of its archived historical result', () => {
  const canonical=readFileSync(new URL('../../docs/release-evidence/portfolio/r07/latest-demo.json',import.meta.url),'utf8');
  assert.equal(readFileSync(new URL('../../web/src/lib/portfolio-demo.generated.json',import.meta.url),'utf8'),canonical);
  const value=JSON.parse(canonical);assert.equal(value.purpose,'historical-local-synthetic-demonstration');
  assert.equal(readFileSync(new URL(`../../docs/release-evidence/portfolio/r07/${value.run_id}.json`,import.meta.url),'utf8'),canonical);
});
test('only the exact running temporary loopback target is accepted', () => {
  assert.equal(assertDisposableTarget(target, expected), 54321);
  for (const alter of [v => v.Config.Labels['com.docker.compose.project'] = 'compose', v => v.Config.Labels['ledgersync.purpose'] = 'another-test', v => v.Id = 'c'.repeat(64), v => v.Image = 'sha256:' + 'c'.repeat(64), v => v.Mounts.push({ Type: 'volume' }), v => v.HostConfig.Tmpfs = {}, v => v.HostConfig.Tmpfs['/another'] = 'rw', v => v.NetworkSettings.Ports['5432/tcp'][0].HostIp = '0.0.0.0', v => v.State.Running = false]) {
    const value = structuredClone(target); alter(value); assert.throws(() => assertDisposableTarget(value, expected));
  }
  assert.throws(() => assertDisposableTarget(target, { ...expected, project: 'compose' }));
  assert.throws(() => assertDisposableTarget(target, { ...expected, port: 54322 }));
  const stopped=structuredClone(target);stopped.State.Running=false;assert.equal(assertDisposableTarget(stopped,expected,{allowStopped:true}),54321);
  const paused=structuredClone(target);paused.State.Paused=true;assert.throws(()=>assertDisposableTarget(paused,expected));assert.equal(assertDisposableTarget(paused,expected,{allowPaused:true}),54321);
  const desktopPaused=structuredClone(paused);desktopPaused.NetworkSettings.Ports={};desktopPaused.HostConfig.PortBindings={'5432/tcp':[{HostIp:'127.0.0.1',HostPort:''}]};
  assert.equal(assertDisposableTarget(desktopPaused,{...expected,port:54321},{allowPaused:true}),54321);
  assert.throws(()=>assertDisposableTarget(desktopPaused,expected,{allowPaused:true}));
  for(const alter of [v=>v.HostConfig.PortBindings['5432/tcp'][0].HostIp='0.0.0.0',v=>v.HostConfig.PortBindings['5432/tcp'][0].HostPort='54322',v=>v.State.Paused=false]){const value=structuredClone(desktopPaused);alter(value);assert.throws(()=>assertDisposableTarget(value,{...expected,port:54321},{allowPaused:true}));}
});

test('artifact verification rejects tampering and mismatched source or migration claims', async () => {
  const root=mkdtempSync(join(tmpdir(),'ledger-demo-path-'));
  try {
    const manifest=JSON.parse(readFileSync(new URL('../../contracts/demo/run-manifest.v1.example.json',import.meta.url),'utf8'));
    Object.assign(manifest,{purpose:'run-evidence',source:{commit:'a'.repeat(40),manifest_sha256:'',dirty:true},migration:{latest:'000001_fixture.up.sql',count:1,manifest_sha256:''}});
    manifest.images[0].digest='sha256:'+'b'.repeat(64);
    const source={head:manifest.source.commit,dirty:true,files:[{path:'fixture.go',sha256:'c'.repeat(64)}]},migration={applied:['000001_fixture.up.sql'],source_migrations:[{path:'000001_fixture.up.sql',sha256:'d'.repeat(64)}]};
    const bind=()=>{manifest.artifacts=[];for(const [name,value,field]of [['source-manifest.json',source,'source'],['migration-manifest.json',migration,'migration']]){const content=JSON.stringify(value);writeFileSync(join(root,name),content);const digest=createHash('sha256').update(content).digest('hex');manifest[field].manifest_sha256=digest;manifest.artifacts.push({path:name,sha256:digest,bytes:Buffer.byteLength(content)});}};
    bind();assert.equal((await verifyDemoArtifacts(manifest,root)).verified,true);
    writeFileSync(join(root,'source-manifest.json'),'tampered');await assert.rejects(verifyDemoArtifacts(manifest,root),/integrity/);
    source.head='e'.repeat(40);bind();await assert.rejects(verifyDemoArtifacts(manifest,root),/Source provenance/);
    source.head=manifest.source.commit;bind();manifest.migration.count=2;await assert.rejects(verifyDemoArtifacts(manifest,root),/migration provenance/);
    manifest.migration.count=1;migration.source_migrations=[];bind();await assert.rejects(verifyDemoArtifacts(manifest,root),/migration source/);
  } finally {assert.equal(dirname(root),tmpdir());assert.match(basename(root),/^ledger-demo-path-/);rmSync(root,{recursive:true,force:true});}
});
test('an exact runtime pass is required; empty, skipped, failed and foreign tests fail', () => {
  const log = JSON.stringify({ Action: 'pass', Test: 'TestReplay' });
  assert.equal(scenarioResult(log, 'TestReplay', 0).status, 'passed');
  for (const [output, code] of [['', 0], [log, 1], [log, null], [JSON.stringify({ Action: 'pass', Test: 'TestOther' }), 0], [log + '\n' + JSON.stringify({ Action: 'skip', Test: 'TestReplay/sub' }), 0]]) assert.equal(scenarioResult(output, 'TestReplay', code).status, 'failed');
  assert.throws(() => scenarioResult('not-json', 'TestReplay', 0));
  assert.throws(() => scenarioResult('x'.repeat(8 * 1024 * 1024 + 1), 'TestReplay', 0));
});
test('artifact paths stay under the bundle and reject traversal and symlinks', () => {
  const root = mkdtempSync(join(tmpdir(), 'ledger-demo-path-'));
  try {
    writeFileSync(join(root, 'proof.json'), '{}'); assert.equal(bundlePath(root, 'proof.json'), join(root, 'proof.json'));
    for (const name of ['../outside', '/outside', 'C:/outside', 'nested/../proof.json', 'proof.json/child', 'nested//file']) assert.throws(() => bundlePath(root, name));
    mkdirSync(join(root, 'nested')); symlinkSync(join(root, 'nested'), join(root, 'link'), process.platform === 'win32' ? 'junction' : 'dir'); assert.throws(() => bundlePath(root, 'link/proof.json'));
  } finally { assert.equal(dirname(root), tmpdir()); assert.match(basename(root), /^ledger-demo-path-/); rmSync(root, { recursive: true, force: true }); }
});
