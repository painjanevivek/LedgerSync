import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname, basename } from 'node:path';
import { createHmac, randomUUID } from 'node:crypto';
import { ReceiverStore } from './receiver.ts';
import { IntentStore } from './intents.ts';

test('signed receiver persists deduplication, rejects tampering, and never regresses aggregate versions', () => {
  const root = mkdtempSync(join(tmpdir(), 'ledger-example-')), key = Buffer.alloc(32, 19), now = Date.now();
  const path = join(root, 'receiver.sqlite'); let receiver = new ReceiverStore(path, { local: key });
  const account = randomUUID();
  const envelope = (version: string) => ({ event_id: randomUUID(), event_type: 'account.balance.changed.v1', account_id: account, balance_version: version });
  const signed = (body: unknown, stamp = new Date(now).toISOString()) => { const raw = Buffer.from(JSON.stringify(body)), id = randomUUID(), v = body as { event_id: string; event_type: string }; return { raw, headers: { 'x-ledgersync-delivery-id': id, 'x-ledgersync-event-id': v.event_id, 'x-ledgersync-event-type': v.event_type, 'x-ledgersync-key-id': 'local', 'x-ledgersync-timestamp': stamp, 'x-ledgersync-signature': 'v1=' + createHmac('sha256', key).update(`${stamp}.${id}.`).update(raw).update('.local').digest('hex') } }; };
  try {
    const newest = signed(envelope('9007199254740993'));
    assert.equal(receiver.accept(newest.raw, newest.headers, now).outcome, 'applied');
    receiver.close(); receiver = new ReceiverStore(path, { local: key });
    assert.equal(receiver.accept(newest.raw, newest.headers, now).outcome, 'duplicate');
    const stale = signed(envelope('2')); assert.equal(receiver.accept(stale.raw, stale.headers, now).outcome, 'stale');
    assert.equal(receiver.database.prepare('SELECT version FROM projections').get()!.version, '9007199254740993');
    const count = receiver.database.prepare('SELECT sum(effect_applied) AS n FROM events').get()!; assert.equal(Number(count.n), 1);
    assert.equal(receiver.accept(newest.raw, { ...newest.headers, 'x-ledgersync-event-id': randomUUID() }, now).status, 400);
    const altered = Buffer.from(newest.raw); altered[altered.length - 2] ^= 1; assert.equal(receiver.accept(altered, newest.headers, now).status, 401);
    const old = signed(envelope('3'), new Date(now - 301_000).toISOString()); assert.equal(receiver.accept(old.raw, old.headers, now).status, 401);
    const equal = signed(envelope('9007199254740993')); assert.equal(receiver.accept(equal.raw, equal.headers, now).status, 409);
    const wrong = signed({ ...JSON.parse(newest.raw.toString()), balance_version: '4' }); assert.equal(receiver.accept(wrong.raw, wrong.headers, now).status, 409);
  } finally { receiver.close(); assert.equal(dirname(root), tmpdir()); assert.match(basename(root), /^ledger-example-/); rmSync(root, { recursive: true, force: true }); }
});

test('complete normalized intent survives restart and cannot change under the original key', () => {
  const root = mkdtempSync(join(tmpdir(), 'ledger-example-')), path = join(root, 'intent.sqlite'); let store = new IntentStore(path);
  try {
    const intent = { key: randomUUID(), principal: 'operator', baseUrl: 'http://127.0.0.1:12345/api', operationId: 'createTransfer', path: '/transfers', body: { amount: '0.01', currency: 'USD' } };
    const prepared = store.prepare(intent); store.close(); store = new IntentStore(path);
    assert.equal(store.get(intent.key).bodyJSON, prepared.bodyJSON);
    assert.throws(() => store.prepare({ ...intent, body: { amount: '0.02', currency: 'USD' } }));
    assert.throws(() => store.prepare({ ...intent, key: randomUUID(), body: { amount: 0.01 } }));
    assert.equal(store.get(intent.key).state, 'prepared');
    store.database.prepare('UPDATE intents SET body_json=? WHERE key=?').run(JSON.stringify({ amount: '0.09', currency: 'USD' }), intent.key);
    assert.throws(() => store.get(intent.key), /integrity check/);
  } finally { store.close(); assert.equal(dirname(root), tmpdir()); assert.match(basename(root), /^ledger-example-/); rmSync(root, { recursive: true, force: true }); }
});
