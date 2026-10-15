import { spawn, execFileSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { repository, captureCandidateSource, sourceIdentity } from './candidate-source.mjs';

// A dirty-tree review candidate is separate from the committed release gate.
// All images bind the complete source manifest. Normal service tags are never used.
const stamp = new Date().toISOString().replace(/\D/g, '').slice(0, 14);
const suffix = randomBytes(4).toString('hex');
const project = `ledgersync-acceptance-${stamp}-${suffix}`;
const phase = process.argv[2];
if (!/^r(?:13|17)-\d{8}$/.test(phase ?? '') || process.argv.length !== 3) throw new Error('Usage: node scripts/portfolio/run-release-candidate.mjs r13-YYYYMMDD (or r17)');
const root = join(repository, 'data/portfolio', phase, `candidate-${stamp}-${suffix}`);
const save = (name, value) => writeFileSync(join(root, name), typeof value === 'string' ? value : JSON.stringify(value, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
const cleanEnvironment = Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith('LEDGERSYNC_') && key !== 'POSTGRES_PASSWORD' && !key.startsWith('PG')));
const docker = (...args) => execFileSync('docker', args, { cwd: repository, env: cleanEnvironment, encoding: 'utf8', windowsHide: true, timeout: 120000, maxBuffer: 8 * 1024 * 1024 }).trim();
const normal = () => {
  const ids = docker('ps', '-aq', '--filter', 'label=com.docker.compose.project=compose').split(/\s+/).filter(Boolean).sort();
  return ids.map(id => { const c = JSON.parse(docker('inspect', id))[0]; return { id, image: c.Image, running: c.State.Running, mounts: c.Mounts }; });
};
const before = normal();
if (before.length < 5 || !before.some(c => c.running)) throw new Error('Preserved normal stack must be available');
if (docker('ps', '-aq', '--filter', `label=com.docker.compose.project=${project}`)) throw new Error('Candidate project collision');
const candidate = captureCandidateSource(root);
save('normal-before.json', before);
const controller = new AbortController();
const timer = setTimeout(() => controller.abort(), 45 * 60 * 1000);
process.once('SIGINT', () => controller.abort()); process.once('SIGTERM', () => controller.abort());
const secrets = [];
async function run(executable, args, name, options = {}) {
  const result = await new Promise((resolve, reject) => {
    const child = spawn(executable, args, { cwd: repository, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'], signal: controller.signal, ...options });
    let output = ''; const collect = data => { output += data; if (Buffer.byteLength(output) > 24 * 1024 * 1024) child.kill(); };
    child.stdout.on('data', collect); child.stderr.on('data', collect);
    child.once('error', reject); child.once('close', code => resolve({ code, output }));
  });
  save(name, secrets.reduce((text, secret) => text.replaceAll(secret, '[REDACTED]'), result.output));
  if (result.code !== 0) throw new Error(`Candidate command failed: ${name}`);
}
const images = {}, targets = [];
let configFile, failure, cleanupFailure;
const compose = (...args) => ['compose', '-p', project, '-f', configFile, ...args];
function verifyTarget(target) {
  const c = JSON.parse(docker('inspect', target.id))[0];
  if (c.Id !== target.id || c.Image !== target.image || c.Config.Labels?.['com.docker.compose.project'] !== project || c.Config.Labels?.['ledgersync.candidate'] !== candidate.manifest_sha256 || c.Mounts.some(m => m.Type !== 'tmpfs' && !(m.Type === 'bind' && !m.RW))) throw new Error('Candidate target identity/storage changed');
  for (const bindings of Object.values(c.HostConfig.PortBindings ?? {})) for (const bind of bindings ?? []) if (bind.HostIp !== '127.0.0.1') throw new Error('Candidate port is not loopback');
  return c;
}
try {
  for (const service of ['api', 'outbox-worker', 'migrate', 'web']) {
    const tag = `ledgersync-review-${service}:${stamp}-${suffix}`;
    const dockerfile = service === 'migrate' ? 'api' : service;
    const args = ['buildx', 'build', '--builder', 'desktop-linux', '--load', '--progress', 'plain', '--label', `org.opencontainers.image.revision=${candidate.identity.head}`, '--label', `ledgersync.source-manifest=${candidate.manifest_sha256}`, '--tag', tag, '--file', join(candidate.snapshot, `deploy/docker/${dockerfile}.Dockerfile`)];
    if (service === 'migrate') args.push('--target', 'database-tooling');
    if (service !== 'web') args.push('--build-arg', 'GO_BUILD_GODEBUG=tlsmlkem=0');
    await run('docker', [...args, candidate.snapshot], `build-${service}.txt`);
    const image = JSON.parse(docker('image', 'inspect', tag))[0];
    if (image.Config.Labels?.['ledgersync.source-manifest'] !== candidate.manifest_sha256) throw new Error('Candidate image provenance mismatch');
    images[service] = { tag, id: image.Id, user: image.Config.User, size: image.Size };
  }
  save('images.json', images);
  const names = ['POSTGRES_PASSWORD', 'LEDGERSYNC_API_DATABASE_PASSWORD', 'LEDGERSYNC_WORKER_DATABASE_PASSWORD', 'LEDGERSYNC_SESSION_SECRET', 'LEDGERSYNC_CONSISTENCY_SIGNING_KEY', 'LEDGERSYNC_BFF_ASSERTION_SECRET', 'LEDGERSYNC_WEB_SESSION_SECRET', 'LEDGERSYNC_DEVELOPMENT_API_TOKEN'];
  const environment = names.map(name => { const value = randomBytes(32).toString('hex'); secrets.push(value); return `${name}=${value}`; });
  environment.push('LEDGERSYNC_INITIALIZATION_MODE=demo'); save('runtime.env', environment.join('\n') + '\n');
  save('recovery-evidence-index.json', { format_version: 'ledgersync-recovery-evidence-index/v1', generated_at_utc: new Date().toISOString(), latest_backup: null, latest_restore: null, retention: { valid_backup_count: 0, ignored_entry_count: 0, configured_keep_count: 5 } });
  const rendered = JSON.parse(docker('compose', '-p', project, '--profile', 'collaboration', '--env-file', join(root, 'runtime.env'), '-f', join(repository, 'deploy/compose/docker-compose.yml'), 'config', '--format', 'json'));
  for (const [name, service] of Object.entries(rendered.services)) {
    delete service.build; delete service.profiles;
    service.labels = { ...service.labels, 'ledgersync.candidate': candidate.manifest_sha256 };
    service.restart = 'no'; service.cpus = 1; service.mem_limit = name === 'web' || name === 'web-peer' ? '768m' : '512m';
    if (images[name]) service.image = images[name].id;
    if (name === 'web-peer') service.image = images.web.id;
    if (name === 'postgres' || name === 'redis') {
      service.volumes = [];
      service.tmpfs.push(name === 'postgres' ? '/var/lib/postgresql/data:rw,size=512m,uid=70,gid=70,mode=0700' : '/data:rw,size=128m,uid=999,gid=1000,mode=0700');
    }
    if (name === 'migrate' || name === 'demo-seed') service.tmpfs.push('/var/lib/postgresql/data:rw,size=1m,uid=70,gid=70,mode=0700');
    if (name === 'api') service.volumes = [{ type: 'bind', source: join(root, 'recovery-evidence-index.json'), target: '/run/ledgersync/recovery/recovery-evidence-index.json', read_only: true }];
    if (name === 'demo-seed') for (const mount of service.volumes) mount.source = join(candidate.snapshot, 'deploy/compose', mount.target.endsWith('demo-seed.sql') ? 'demo-seed.sql' : 'local-bootstrap.sql');
    if (name === 'web' || name === 'web-peer') {
      const port = name === 'web' ? 3102 : 3103;
      service.ports = [{ target: 3000, published: String(port), host_ip: '127.0.0.1', protocol: 'tcp' }];
      service.environment.LEDGERSYNC_PUBLIC_ORIGIN = `http://127.0.0.1:${port}`;
      service.healthcheck.test = ['CMD', 'node', '-e', `const r=require('node:http').get('http://127.0.0.1:3000/',{headers:{host:'127.0.0.1:${port}'}},r=>{r.resume();if(r.statusCode!==200)process.exit(1)});r.setTimeout(2000,()=>{r.destroy();process.exit(1)});r.on('error',()=>process.exit(1))`];
    }
  }
  rendered.volumes = {}; configFile = join(root, 'compose.private.json'); save('compose.private.json', rendered);
  await run('docker', compose('up', '-d', '--no-build', '--wait', '--wait-timeout', '180'), 'startup.txt');
  const ids = docker(...compose('ps', '-aq')).split(/\s+/).filter(Boolean);
  for (const id of ids) { const c = JSON.parse(docker('inspect', id))[0]; const target = { id: c.Id, image: c.Image, service: c.Config.Labels['com.docker.compose.service'] }; targets.push(target); verifyTarget(target); }
  save('targets.json', targets);
  save('resources.json', docker('stats', '--no-stream', '--format', '{{json .}}', ...targets.filter(t => verifyTarget(t).State.Running).map(t => t.id)).split(/\r?\n/).map(line => JSON.parse(line)));
  const browserOutput = join(root, 'browser-artifacts');
  const env = { ...process.env, PLAYWRIGHT_HTML_OUTPUT_DIR: join(root, 'browser-report'), LEDGERSYNC_SYSTEM_ARTIFACT_ROOT: browserOutput, LEDGERSYNC_SYSTEM_ALLOW_LEDGER_MUTATION: 'true', LEDGERSYNC_SYSTEM_ISOLATED_PROJECT: 'true', LEDGERSYNC_SYSTEM_WEB_URL: 'http://127.0.0.1:3102', LEDGERSYNC_SYSTEM_PEER_WEB_URL: 'http://127.0.0.1:3103', LEDGERSYNC_SYSTEM_COMPOSE_PROJECT: project, LEDGERSYNC_SYSTEM_SEEDED_SOURCE_ACCOUNT_ID: '10000000-0000-4000-8000-000000000001', LEDGERSYNC_SYSTEM_RUN_ID: `release-${suffix}` };
  await run(process.platform === 'win32' ? 'pwsh' : 'npm', process.platform === 'win32' ? ['-NoProfile', '-Command', 'npm run test:e2e:real-stack -- --output "$env:LEDGERSYNC_SYSTEM_ARTIFACT_ROOT"'] : ['run', 'test:e2e:real-stack', '--', '--output', browserOutput], 'browser-real-stack.txt', { cwd: join(repository, 'web'), env });
  const goEnvironment = { ...cleanEnvironment, GOMAXPROCS: '2', LEDGERSYNC_SYSTEM_WEB_URL: 'http://127.0.0.1:3102', LEDGERSYNC_SYSTEM_IDEMPOTENCY_KEY: `candidate-go-${suffix}` };
  await run('go', ['test', './tests/system', '-count=1', '-v'], 'go-real-stack.txt', { cwd: candidate.snapshot, env: goEnvironment });
  for (const target of targets) verifyTarget(target);
  await run('docker', compose('restart', 'api'), 'api-restart.txt');
  await run('docker', compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '180', 'api'), 'api-restart-ready.txt');
  await run('go', ['test', './tests/system', '-run', '^TestRealBFFSameKeyReplayAfterAPIProcessRestart$', '-count=1', '-v'], 'go-restart-replay.txt', { cwd: candidate.snapshot, env: { ...goEnvironment, LEDGERSYNC_SYSTEM_EXPECT_RESTART_REPLAY: 'true' } });
  save('resources-after.json', docker('stats', '--no-stream', '--format', '{{json .}}', ...targets.filter(t => verifyTarget(t).State.Running).map(t => t.id)) + '\n');
} catch (error) { failure = error.message; }
finally {
  clearTimeout(timer);
  try {
    // Even a partial startup is owned only by this random project and manifest.
    const ids = docker('ps', '-aq', '--filter', `label=com.docker.compose.project=${project}`).split(/\s+/).filter(Boolean);
    for (const id of ids) {
      const c = JSON.parse(docker('inspect', id))[0]; const target = { id: c.Id, image: c.Image };
      verifyTarget(target); docker('rm', '-f', id);
    }
    for (const id of docker('network', 'ls', '-q', '--filter', `label=com.docker.compose.project=${project}`).split(/\s+/).filter(Boolean)) {
      const n = JSON.parse(docker('network', 'inspect', id))[0];
      if (n.Labels?.['com.docker.compose.project'] !== project || Object.keys(n.Containers ?? {}).length) throw new Error('Network cleanup identity mismatch');
      docker('network', 'rm', id);
    }
    if (docker('ps', '-aq', '--filter', `label=com.docker.compose.project=${project}`)) throw new Error('Candidate cleanup incomplete');
  } catch (error) { cleanupFailure = error.message; }
  const after = normal(); save('normal-after.json', after);
  const sourcePreserved = JSON.stringify(sourceIdentity()) === JSON.stringify(candidate.identity);
  const normalPreserved = JSON.stringify(before) === JSON.stringify(after);
  const status = failure || cleanupFailure || !sourcePreserved || !normalPreserved ? 'failed' : 'verified_local';
  save('result.json', { status, project, source_manifest_sha256: candidate.manifest_sha256, images, failure, cleanup_failure: cleanupFailure, source_preserved: sourcePreserved, normal_preserved: normalPreserved, completed_at: new Date().toISOString(), limits: ['Local synthetic full-stack candidate; no cloud or external identity provider qualification.', 'Resource values are finite samples, not peak or sustained HTTP capacity measurements.', 'Candidate images and private evidence are retained. Temporary runtime data is removed.'] });
  console.log(JSON.stringify({ root, status, failure, cleanupFailure, sourcePreserved, normalPreserved }));
  process.exitCode = status === 'verified_local' ? 0 : 1;
}
