import { randomBytes } from 'node:crypto';
import { appendFileSync, lstatSync, existsSync, realpathSync } from 'node:fs';
import { dirname, isAbsolute, resolve } from 'node:path';

export type ReceiverTrace = { name: 'webhook.receiver'; trace_id: string; span_id: string; parent_span_id: string; links: never[]; status: 'ok' | 'error' };

// Opaque diagnostics never authorize a delivery. Signature/replay checks run
// independently over the original bytes. No body, key or business ID is saved.
export function receiverTrace(parent: unknown, status: number): ReceiverTrace | null {
  if (typeof parent !== 'string' || !/^00-[a-f0-9]{32}-[a-f0-9]{16}-0[01]$/.test(parent)) return null;
  const [, traceID, parentID] = parent.split('-');
  if (/^0+$/.test(traceID) || /^0+$/.test(parentID)) return null;
  return { name: 'webhook.receiver', trace_id: traceID, span_id: randomBytes(8).toString('hex'), parent_span_id: parentID, links: [], status: status >= 400 ? 'error' : 'ok' };
}

export function privateTraceSink(path: string, statePath: string): (span: ReceiverTrace) => void {
  const selected = resolve(path), state = resolve(statePath);
  if (!isAbsolute(path) || selected === state || dirname(selected) !== dirname(state) || !selected.endsWith('.trace.jsonl') || realpathSync(dirname(selected)) !== dirname(selected)) throw new Error('Diagnostic file must be a separate protected sibling of receiver state');
  let count = 0;
  return span => {
    const encoded = JSON.stringify(span) + '\n';
    if (existsSync(selected)) {
      const info = lstatSync(selected);
      if (!info.isFile() || info.size + Buffer.byteLength(encoded) > 256 * 1024) throw new Error('Diagnostic output is unavailable');
    }
    if (++count > 1000) throw new Error('Diagnostic span limit reached');
    appendFileSync(selected, encoded, { mode: 0o600 });
  };
}
