import { readFileSync, existsSync, statSync } from 'node:fs';
import { resolve, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const names = ['README.md', ...['README','quickstart','demo-guide','architecture','case-study','limitations','maintainer-guide','statement-reconciliation','reservations','audit-remediation','human-acceptance','tracing','failure-demonstration','financial-model','capacity'].map(n => `docs/portfolio/${n}.md`)];
names.push('deploy/production/README.md','contracts/balance-events.md');
names.push('docs/portfolio/branch-consolidation.md','docs/runbooks/database-capability-boundary.md','docs/runbooks/tenant-rls-rollout.md');
names.push('docs/portfolio/http-capacity-coordinator.md','docs/plans/r09-http-capacity-and-audit-continuation.md');
const failures = [], links = [], commands = [];
const read = name => readFileSync(resolve(root, name), 'utf8').replace(/^\uFEFF/, '');
const anchor = text => text.toLowerCase().replace(/[^\p{L}\p{N}\s_-]/gu, '').replace(/ /g, '-');
for (const name of names) {
  const source = read(name), lines = source.split(/\r?\n/);
  let fence = false;
  for (const line of lines) {
    if (line.startsWith('```')) { fence = !fence; continue; }
    if (!fence) continue;
    const node = /^node (scripts\/[^\s]+\.mjs)(?:\s|$)/.exec(line);
    const powershell = /^\.\\(scripts\\[^\s]+\.ps1)$/.exec(line);
    const target = node?.[1] ?? powershell?.[1].replaceAll('\\','/');
    if (target) { if (!existsSync(join(root,target))) failures.push(`${name}: missing command ${target}`); commands.push({document:name,command:line,entry:target}); }
  }
  if (fence) failures.push(`${name}: unclosed code block`);
  for (const match of source.matchAll(/!?\[[^\]\n]*\]\(([^)\n]+)\)/g)) {
    const url = match[1];
    if (/^(?:https?:|mailto:)/.test(url)) continue;
    const [path, fragment] = url.split('#'), target = path ? resolve(dirname(join(root,name)),decodeURIComponent(path)) : join(root,name);
    if (!existsSync(target)) { failures.push(`${name}: missing link ${url}`); continue; }
    if (fragment && target.endsWith('.md')) {
      const headings = [...readFileSync(target,'utf8').matchAll(/^#{1,6} (.+)$/gm)].map(h => anchor(h[1]));
      if (!headings.includes(fragment)) failures.push(`${name}: missing anchor ${url}`);
    }
    links.push({document:name,target:url});
  }
}
const data = JSON.parse(read('docs/portfolio/assets/r09-capacity.json')), report = read(data.source_report);
const rows = [...report.matchAll(/^\| ([123]) \| 3000 \| 2640 \| ([\d.]+) \/ ([\d.]+) \/ ([\d.]+) \| ([\d.]+) \|/gm)];
if (rows.length !== 3 || rows.some((row,i)=>Number(row[3])!==data.p95_ms[i]||Number(row[5])!==data.steady_oldest_pending_seconds[i])) failures.push('Chart values differ from the archived capacity table');
for (const row of data.calibrations) if (!report.includes(row.queue_age_seconds.map(v=>v.toFixed(2)).join(', '))) failures.push(`Chart calibration differs: ${row.name}`);
if (!report.includes(data.source_manifest_sha256) || !report.includes('8.8')) failures.push('Chart source identity or posting-rate provenance missing');
if (statSync(join(root,'docs/portfolio/assets/r09-capacity.svg')).size<1000) failures.push('Chart SVG is absent or empty');
const architecture = read('docs/portfolio/architecture.md');
if ([...architecture.matchAll(/^```mermaid$/gm)].length !== 5) failures.push('Architecture needs one trust diagram and four sequences');
for (const [term,path] of [['controlled_submit_transfer_v1','internal/platform/db/transfer_repository.go'],['controlled_claim_outbox_v1','internal/platform/db/outbox_repository.go'],['controlled_complete_outbox_v1','internal/platform/db/outbox_repository.go']]) if (!architecture.includes(term)||!read(path).includes(term)) failures.push(`Diagram command is not source-backed: ${term}`);
const result={status:failures.length?'failed':'verified_local',documents:names.length,local_links:links.length,command_entries:commands.length,source_backed_diagrams:5,chart_values_match_archived_source:!failures.some(f=>f.startsWith('Chart')),failures};
console.log(JSON.stringify(result,null,2));
if(failures.length)process.exitCode=1;
