import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { randomUUID } from 'node:crypto';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname, basename } from 'node:path';
import { IntentStore } from './intents.ts';
import { PreparedClient } from './client.ts';

test('lost responses and explicit HTTP errors retain one persisted intent without automatic retries', async () => {
  const root = mkdtempSync(join(tmpdir(), 'ledger-example-')); let calls = 0; let status = 201;
  const keys: string[] = [], bodies: string[] = [];
  const server = createServer(async (req, res) => { calls++; keys.push(String(req.headers['idempotency-key'])); const chunks = []; for await (const chunk of req) chunks.push(Buffer.from(chunk)); bodies.push(Buffer.concat(chunks).toString()); res.writeHead(status, { 'Content-Type': 'application/json', 'Retry-After': '2', 'Idempotent-Replay': String(calls > 1) }).end(JSON.stringify({ transfer_id: 'synthetic-http-fixture' })); });
  server.listen(0, '127.0.0.1'); await once(server, 'listening'); const address = server.address(); assert.ok(address && typeof address !== 'string');
  const baseUrl = `http://127.0.0.1:${address.port}/api`, key = randomUUID(); let store = new IntentStore(join(root, 'intents.sqlite'));
  try {
    let lose = true; const binding = { baseUrl, token: 'synthetic-own-principal-token-32-bytes', fetch: (async (...args: Parameters<typeof fetch>) => { const response = await fetch(...args); if (lose) { lose = false; await response.arrayBuffer(); throw new TypeError('synthetic lost response after server commit'); } return response; }) as typeof fetch };
    let client = new PreparedClient(store, { operator: binding });
    client.prepare({ key, principal: 'operator', baseUrl, operationId: 'createTransfer', path: '/transfers', body: { amount: '0.01', currency: 'USD' } });
    assert.equal((await client.send(key)).kind, 'unknown'); assert.equal(calls, 1);
    const original = store.get(key).bodyJSON; store.close(); store = new IntentStore(join(root, 'intents.sqlite')); client = new PreparedClient(store, { operator: binding });
    const retry = await client.send(key); assert.equal(retry.kind, 'response'); assert.equal(retry.kind === 'response' && retry.replayed, true);
    for (status of [400, 403, 409, 429, 503]) { const before: number = calls; const response = await client.send(key); assert.equal(response.kind, 'response'); assert.equal(response.kind === 'response' && response.status, status); assert.equal(calls, before + 1); if (status === 429) assert.equal(response.kind === 'response' && response.retryAfterSeconds, 2); }
    assert.ok(keys.every(value => value === key)); assert.ok(bodies.every(value => value === original));
    const before = calls;
    await assert.rejects(new PreparedClient(store, { operator: { ...binding, token: 'another-principal-must-not-reuse-this-command' } }).send(key), /Original credential/);
    assert.equal(calls, before);
  } finally { store.close(); server.closeAllConnections(); await new Promise<void>(resolve => server.close(() => resolve())); assert.equal(dirname(root), tmpdir()); assert.match(basename(root), /^ledger-example-/); rmSync(root, { recursive: true, force: true }); }
});
