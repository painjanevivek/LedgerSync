import { execFileSync } from 'node:child_process';
import { readFileSync, readlinkSync, realpathSync } from 'node:fs';
import { resolve } from 'node:path';

// Query only a live child owned by the coordinator. This never sends a signal.
export function sampleOwnedProcess(child, executable) {
  if (!child || !Number.isSafeInteger(child.pid) || child.pid < 1 || child.exitCode !== null || child.signalCode !== null) return { status: 'unavailable', reason: 'owned_child_not_running' };
  try {
    let value;
    const expected = realpathSync(executable);
    if (process.platform === 'win32') {
      const script = `$probe=Get-Process -Id ${child.pid} -ErrorAction Stop; [ordered]@{pid=$probe.Id;cpu_seconds=$probe.TotalProcessorTime.TotalSeconds;rss_bytes=$probe.WorkingSet64;created=$probe.StartTime.ToUniversalTime().Ticks.ToString();path=$probe.Path}|ConvertTo-Json -Compress`;
      value = JSON.parse(execFileSync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', script], { encoding: 'utf8', windowsHide: true, timeout: 3000, maxBuffer: 8192 }));
      if (resolve(value.path).toLowerCase() !== resolve(expected).toLowerCase()) throw Error('Wrong process executable');
    } else if (process.platform === 'linux') {
      if (realpathSync(readlinkSync(`/proc/${child.pid}/exe`)) !== expected) throw Error('Wrong process executable');
      const raw = readFileSync(`/proc/${child.pid}/stat`, 'utf8');
      const created = raw.slice(raw.lastIndexOf(')') + 2).split(' ')[19];
      const [cpu, rss] = execFileSync('ps', ['-p', String(child.pid), '-o', 'cputimes=,rss='], { encoding: 'utf8', timeout: 1000, maxBuffer: 8192 }).trim().split(/\s+/).map(Number);
      value = { pid: child.pid, cpu_seconds: cpu, rss_bytes: rss * 1024, created };
    } else return { status: 'unavailable', reason: 'platform_not_qualified' };
    if (value.pid !== child.pid || !Number.isFinite(value.cpu_seconds) || value.cpu_seconds < 0 || !Number.isSafeInteger(value.rss_bytes) || value.rss_bytes < 1 || !/^\d+$/.test(value.created)) throw Error('Invalid process sample');
    return { status: 'sampled', pid: value.pid, cpu_seconds: value.cpu_seconds, rss_bytes: value.rss_bytes, creation_identity: value.created };
  } catch {
    return { status: 'unavailable', reason: 'owned_process_probe_failed' };
  }
}
