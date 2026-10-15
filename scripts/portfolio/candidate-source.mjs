import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync, mkdirSync, existsSync, lstatSync, realpathSync, copyFileSync, constants, readdirSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { resolve, dirname, relative, isAbsolute, join } from 'node:path';
import { fileURLToPath } from 'node:url';

export const repository = realpathSync(resolve(dirname(fileURLToPath(import.meta.url)), '../..'));
const digest = value => createHash('sha256').update(value).digest('hex');
const git = (...args) => execFileSync('git', args, { cwd: repository, encoding: 'utf8', windowsHide: true, maxBuffer: 16 * 1024 * 1024 });
const localArtifacts = /^(?:\.impeccable|brag-output|output)\//;

export function sourceIdentity() {
  const paths = git('ls-files', '--cached', '--others', '--exclude-standard', '-z').split('\0').filter(path => path && existsSync(resolve(repository, path))).sort();
  const files = paths.map(path => {
    if (isAbsolute(path) || path.includes('\\') || path.split('/').some(part => !part || part === '.' || part === '..')) throw new Error('Unsafe source path');
    const name = resolve(repository, path), stat = lstatSync(name);
    if (!stat.isFile()) throw new Error('Regular source files required');
    return { path, bytes: stat.size, sha256: digest(readFileSync(name)), scope: localArtifacts.test(path) ? 'preserved-local-artifact' : 'candidate-source' };
  });
  return { head: git('rev-parse', 'HEAD').trim(), index_sha256: digest(git('ls-files', '--stage', '-z')), dirty: git('status', '--porcelain').trim().length > 0, files };
}

export function captureCandidateSource(root) {
  const allowed = resolve(repository, 'data/portfolio'), offset = relative(allowed, root);
  if (isAbsolute(offset) || offset.startsWith('..') || !/^r(?:1[3-7])-\d{8}[\\/]candidate-\d{14}-[a-f0-9]{8}$/.test(offset)) throw new Error('Exact private candidate root required');
  mkdirSync(root, { recursive: true });
  if (realpathSync(root) !== root) throw new Error('Candidate output symlinks are forbidden');
  const identity = sourceIdentity(), manifest = JSON.stringify({ captured_at: new Date().toISOString(), ...identity }, null, 2) + '\n';
  writeFileSync(join(root, 'source-manifest.json'), manifest, { flag: 'wx', mode: 0o600 });
  const snapshot = join(root, 'source'); mkdirSync(snapshot);
  for (const file of identity.files.filter(file => file.scope === 'candidate-source')) {
    const destination = join(snapshot, file.path); mkdirSync(dirname(destination), { recursive: true });
    copyFileSync(join(repository, file.path), destination, constants.COPYFILE_EXCL);
    if (digest(readFileSync(destination)) !== file.sha256) throw new Error('Source changed while snapshotting');
  }
  return { root, snapshot, identity, manifest_sha256: digest(manifest) };
}

export function verifyBuildContext(root) {
  const files = [], allowed = new Set(['cmd', 'contracts', 'internal', 'migrations', 'deploy', 'web', 'go.mod', 'go.sum', 'go.work', 'go.work.sum']);
  const walk = (path, prefix = '') => {
    for (const name of readdirSync(path)) {
      const absolute = join(path, name), relativePath = prefix + name, stat = lstatSync(absolute);
      if (stat.isSymbolicLink()) throw new Error('Build context contains a symlink');
      if (stat.isDirectory()) walk(absolute, relativePath + '/');
      else {
        if (!stat.isFile() || !allowed.has(relativePath.split('/')[0]) || /(?:^|\/)(?:\.env[^/]*|[^/]+\.env|node_modules|\.next|\.git|\.agents|\.codex|\.aws|test-results|playwright-report[^/]*|AGENTS\.md|SKILL\.md)(?:\/|$)/.test(relativePath)) throw new Error('Private or unintended file entered the build context');
        if (digest(readFileSync(absolute)) !== digest(readFileSync(join(repository, relativePath)))) throw new Error('Exported source differs from current source');
        files.push({ path: relativePath, bytes: stat.size, sha256: digest(readFileSync(absolute)) });
      }
    }
  };
  walk(root); files.sort((a, b) => a.path.localeCompare(b.path));
  if (!files.some(file => file.path === 'go.mod') || !files.some(file => file.path === 'web/package-lock.json') || !files.some(file => file.path.startsWith('migrations/'))) throw new Error('Required build inputs absent');
  return { status: 'verified_local', files, bytes: files.reduce((sum, file) => sum + file.bytes, 0), manifest_sha256: digest(JSON.stringify(files)), private_runtime_state_included: false };
}
