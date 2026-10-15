// Executed only by the documented disposable qualification. Automated finance
// review here is synthetic test evidence, not a claim that a human reviewed it.
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { dirname, join } from 'node:path';
import { readFileSync, writeFileSync } from 'node:fs';
import { createClient, operations } from '../../../contracts/generated/typescript/ledgersync.ts';
import { loadExampleConfig, principalBindings } from './config.ts';
import { IntentStore } from './intents.ts';
import { PreparedClient } from './client.ts';
import { ReceiverStore } from './receiver.ts';

const [configPath, action] = process.argv.slice(2), config = loadExampleConfig(configPath), root = dirname(config.statePath);
const address = new URL(config.baseUrl);
if (address.hostname !== '127.0.0.1' || address.pathname !== '/api' || process.argv.length !== 4) throw new Error('Qualification requires its own local API allocation');
const bindings = principalBindings(config); let store = new IntentStore(config.statePath); let prepared = new PreparedClient(store, bindings);
const operation = (id: string) => { const value = operations.find(v => v.id === id); assert.ok(value); return value; };
async function call(id: string, path: string, principal = 'operator', body?: unknown, key: string = randomUUID()) {
  const response = await createClient(bindings[principal]).call(operation(id), path, { headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) });
  return { status: response.status, body: await response.json(), replayed: response.headers.get('Idempotent-Replay'), retry: response.headers.get('Retry-After') };
}
async function mutate(id: string, path: string, body: unknown, principal = 'operator', status = 201) {
  const key = randomUUID(); prepared.prepare({ key, principal, baseUrl: config.baseUrl, operationId: id, path, body });
  const response = await prepared.send(key); assert.equal(response.kind, 'response'); if (response.kind !== 'response') throw new Error('Unexpected unknown outcome'); assert.equal(response.status, status, JSON.stringify(response.body)); return response.body as Record<string, any>;
}
try {
  if (action === 'setup') {
    const source = await mutate('createAccount', '/accounts', { display_name: 'Synthetic integration source', external_reference: 'source-' + randomUUID(), category: 'operating', currency: 'INR' });
    const destination = await mutate('createAccount', '/accounts', { display_name: 'Synthetic integration destination', external_reference: 'destination-' + randomUUID(), category: 'operating', currency: 'INR' });
    assert.equal(source.available_minor, '0'); assert.equal(destination.available_minor, '0');
    const requested = await mutate('requestFunding', '/funding-requests', { destination_account_id: source.account_id, amount_minor: '1000', currency: 'INR', external_reference: 'synthetic-' + randomUUID(), evidence_reference: 'synthetic://portfolio/r12' });
    const event = requested.event; assert.ok(event?.funding_event_id);
    const self = await call('approveFundingEvent', `/funding-events/${event.funding_event_id}/approve`, 'operator', { reason: 'Synthetic self-review negative check' }); assert.equal(self.status, 403);
    const read = await call('getFundingEvent', `/funding-events/${event.funding_event_id}`, 'reviewer'); assert.equal(read.status, 200); assert.equal(read.body.status, 'requested'); assert.equal(read.body.amount_minor, '1000');
    const reason = { reason: 'Independent synthetic fixture evidence review; not a human approval claim' }, approvalKey = randomUUID();
    store.prepare({ key: approvalKey, principal: 'reviewer', baseUrl: config.baseUrl, operationId: 'approveFundingEvent', path: `/funding-events/${event.funding_event_id}/approve`, body: reason });
    const approved = await call('approveFundingEvent', `/funding-events/${event.funding_event_id}/approve`, 'reviewer', reason, approvalKey); assert.equal(approved.status, 200); store.record(approvalKey, 'responded', approved.status, JSON.stringify(approved.body));
    const posted = await mutate('postFundingEvent', `/funding-events/${event.funding_event_id}/post`, {}, 'reviewer', 200); assert.equal(posted.event.status, 'posted');
    const webhook = await mutate('registerDeveloperWebhook', '/developer/webhooks', { display_name: 'Synthetic receiver', endpoint_url: 'https://receiver.example.test/hooks', subscribed_events: ['transfer.posted'], signing_key_reference: 'synthetic-example-key', signing_key_id: 'example-key' });
    writeFileSync(join(root, 'workflow.json'), JSON.stringify({ source: source.account_id, destination: destination.account_id, funding: event.funding_event_id, webhook: webhook.webhook_id }), { flag: 'wx', mode: 0o600 });
    console.log(JSON.stringify({ setup: 'passed', account_creation: 'zero', funding: 'independent synthetic review and post', webhook: 'verification scheduled' }));
  } else if (action === 'transfer-once') {
    const flow = JSON.parse(readFileSync(join(root, 'workflow.json'), 'utf8'));
    const webhook = await call('getDeveloperWebhook', `/developer/webhooks/${flow.webhook}`); assert.equal(webhook.status, 200); assert.equal(webhook.body.status, 'active');
    const key = randomUUID(); writeFileSync(join(root, 'transfer-key.txt'), key, { flag: 'wx', mode: 0o600 });
    prepared.prepare({ key, principal: 'operator', baseUrl: config.baseUrl, operationId: 'createTransfer', path: '/transfers', body: { source_account_id: flow.source, destination_account_id: flow.destination, amount: '2.50', currency: 'INR' } });
    let discardedStatus = 0;
    const lost = { ...bindings.operator, fetch: (async (...args: Parameters<typeof fetch>) => { const response = await fetch(...args); discardedStatus = response.status; await response.arrayBuffer(); throw new TypeError('synthetic lost response after actual API commit'); }) as typeof fetch };
    prepared = new PreparedClient(store, { ...bindings, operator: lost }); assert.equal((await prepared.send(key)).kind, 'unknown');
    assert.equal(discardedStatus, 201, 'The discarded response must prove an actual successful API commit.');
    console.log(JSON.stringify({ original: 'unknown after actual API response loss', next: 'new process retries persisted key' }));
  } else if (action === 'retry') {
    const flow = JSON.parse(readFileSync(join(root, 'workflow.json'), 'utf8')), key = readFileSync(join(root, 'transfer-key.txt'), 'utf8'), saved = store.get(key);
    assert.equal(saved.state, 'unknown'); const retry = await prepared.send(key); assert.equal(retry.kind, 'response'); if (retry.kind !== 'response') throw new Error('Retry stayed unknown'); assert.equal(retry.status, 201); assert.equal(retry.replayed, true);
    const original = saved.intent.body as Record<string, unknown>;
    const conflict = await call('createTransfer', '/transfers', 'operator', { ...original, amount: '2.51' }, key); assert.equal(conflict.status, 409);
    const invalid = await call('createTransfer', '/transfers', 'operator', { ...original, amount: 2.5 }); assert.equal(invalid.status, 400);
    const denied = await call('createTransfer', '/transfers', 'reader', original); assert.equal(denied.status, 403);
    const source = await call('getAccountBalance', `/accounts/${flow.source}/balance`), destination = await call('getAccountBalance', `/accounts/${flow.destination}/balance`);
    assert.equal(source.status, 200); assert.equal(destination.status, 200); assert.equal(source.body.available_minor, '750'); assert.equal(destination.body.available_minor, '250');
    const reconciliation = await mutate('runReconciliation', '/reconciliation/runs', {}, 'operator', 201); assert.equal(reconciliation.status, 'matched'); assert.equal(reconciliation.mismatch_count, '0');
    let limited = false; for (let i = 0; i < 20; i++) { const response = await call('createTransfer', '/transfers', 'operator', original, key); if (response.status === 429) { assert.ok(Number(response.retry) > 0); limited = true; break; } assert.equal(response.status, 201); }
    assert.equal(limited, true);
    writeFileSync(join(root, 'workflow-result.json'), JSON.stringify({ transfer_id: (retry.body as Record<string, unknown>).transfer_id, source: flow.source, destination: flow.destination, exact_retry: true, conflict: 409, invalid: 400, denied: 403, rate_limited: 429, reconciliation: 'matched', source_minor: '750', destination_minor: '250' }), { flag: 'wx', mode: 0o600 });
    console.log(JSON.stringify({ retry: 'original', exact_minor: true, conflict: 409, invalid: 400, denied: 403, rate_limit: 429, reconciliation: 'matched' }));
  } else if (action === 'receiver-check') {
    assert.ok(config.receiver); const receiver = new ReceiverStore(config.receiver.statePath, { [config.receiver.keyId]: Buffer.from(config.receiver.keyBase64, 'base64') });
    try { const row = receiver.database.prepare('SELECT count(*) AS n,sum(effect_applied) AS effects FROM events').get()!; assert.equal(Number(row.n), 1); assert.equal(Number(row.effects), 1); console.log(JSON.stringify({ durable_events: 1, business_effects: 1 })); } finally { receiver.close(); }
  } else throw new Error('Unknown bounded qualification action');
} finally { store.close(); }
