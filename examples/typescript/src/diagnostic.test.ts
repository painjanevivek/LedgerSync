import test from 'node:test';
import assert from 'node:assert/strict';
import { receiverTrace, privateTraceSink } from './diagnostic.ts';
import { mkdtempSync, readFileSync, writeFileSync, mkdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

test('receiver trace keeps opaque ancestry and independent attempt identities',()=>{
 const parent='00-11111111111111111111111111111111-2222222222222222-01';
 const first=receiverTrace(parent,503)!,second=receiverTrace(parent,204)!;
 assert.equal(first.trace_id,second.trace_id);assert.notEqual(first.span_id,second.span_id);assert.equal(first.parent_span_id,'2222222222222222');assert.equal(first.status,'error');assert.equal(second.status,'ok');
 assert.deepEqual(Object.keys(first).sort(),['links','name','parent_span_id','span_id','status','trace_id']);
 for(const invalid of [undefined,[],parent+',baggage=private',parent.replace('11111111111111111111111111111111','0'.repeat(32)),parent.replace('2222222222222222','0'.repeat(16))])assert.equal(receiverTrace(invalid,204),null);
});

test('private diagnostic sink enforces location, regular file, size and count bounds', () => {
  const root = mkdtempSync(join(tmpdir(), 'ledgersync-receiver-trace-'));
  try {
    const state = join(root, 'receiver.sqlite'), path = join(root, 'receiver.trace.jsonl');
    const span = receiverTrace('00-' + '1'.repeat(32) + '-' + '2'.repeat(16) + '-01', 204)!;
    for (const invalid of ['relative.trace.jsonl', state, join(root, 'outside', 'other.trace.jsonl'), join(root, 'wrong.jsonl')]) {
      assert.throws(() => privateTraceSink(invalid, state));
    }
    const save = privateTraceSink(path, state);
    save(span);
    assert.deepEqual(JSON.parse(readFileSync(path, 'utf8')), span);
    writeFileSync(path, Buffer.alloc(256 * 1024));
    assert.throws(() => save(span), /unavailable/);
    const bounded = privateTraceSink(join(root, 'bounded.trace.jsonl'), state);
    for (let i = 0; i < 1000; i++) bounded(span);
    assert.throws(() => bounded(span), /limit/);
    const directory = join(root, 'directory.trace.jsonl');
    mkdirSync(directory);
    assert.throws(() => privateTraceSink(directory, state)(span), /unavailable/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});
