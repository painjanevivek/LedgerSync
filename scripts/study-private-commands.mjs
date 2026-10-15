// Run only inside the isolated study web container. Do not print credentials.
import { lookup } from 'node:dns/promises';
import { createHash, createHmac, randomUUID } from 'node:crypto';

if (process.env.LEDGERSYNC_DEPLOYMENT_ENV !== 'development' || process.env.LEDGERSYNC_PUBLIC_ORIGIN !== 'http://127.0.0.1:3102') throw new Error('An isolated study web container is required');
const tenant = '00000000-0000-4000-8000-000000000001';
const nodes = [...new Set((await lookup('api', { all: true, family: 4 })).map(item => item.address))].sort();
if (nodes.length !== 2) throw new Error('Two API replicas are required');
const scopes = ['accounts:read', 'accounts:write', 'transfers:read', 'transfers:write', 'reconciliation:read', 'reconciliation:write'];
const requests = [];
let sequence = 0;
async function command(path, body, key = `study-${randomUUID()}`, index = sequence++, retry = 0) {
  const now = Math.floor(Date.now() / 1000);
  const payload = Buffer.from(JSON.stringify({ iss: 'ledgersync-bff', aud: 'ledgersync-private-api', kid: 'current', jti: randomUUID(), sub: 'local-user', tenant_id: tenant, roles: ['tenant:operator'], scopes, authenticated_at: now, iat: now, exp: now + 60 }));
  const assertion = `${payload.toString('base64url')}.${createHmac('sha256', process.env.LEDGERSYNC_BFF_ASSERTION_SECRET).update(payload).digest('base64url')}`;
  const started = performance.now();
  const hash = createHash('sha256').update(key).digest('hex');
  const reference = `${hash.slice(0,8)}-${hash.slice(8,12)}-4${hash.slice(13,16)}-8${hash.slice(17,20)}-${hash.slice(20,32)}`;
  const response = await fetch(`http://${nodes[index % nodes.length]}:8080${path}`, { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${process.env.LEDGERSYNC_PRIVATE_API_TOKEN}`, 'X-LedgerSync-Actor-Assertion': assertion, 'Idempotency-Key': key, 'X-LedgerSync-Request-Reference': reference, 'X-Request-ID': randomUUID() }, body: JSON.stringify(body), signal: AbortSignal.timeout(15000) });
  const result = await response.json();
  requests.push({ path, status: response.status, node: response.headers.get('x-ledgersync-node'), elapsed_ms: performance.now() - started, outcome: result.status ?? result.error?.code, transfer_id: result.transfer_id, run_id: result.run_id });
  if (response.status === 503 && result.error?.code === 'transaction_conflict_retryable' && retry < 3) {
    await new Promise(resolve => setTimeout(resolve, 100 * (retry + 1)));
    return command(path, body, key, index, retry + 1);
  }
  return { status: response.status, body: result };
}
async function account(label) {
  const result = await command('/api/accounts', { display_name: `Study ${label}`, external_reference: `study-${randomUUID()}`, category: 'operating', currency: 'INR' });
  if (result.status !== 201 || !result.body.account_id) throw new Error(`Account creation failed: ${JSON.stringify(result)}`);
  return result.body.account_id;
}
const transfer = (source, destination, amount, key, index) => command('/api/transfers', { source_account_id: source, destination_account_id: destination, amount, currency: 'INR' }, key, index);
const source = '10000000-0000-4000-8000-000000000001';
const hot = await account('hot');
const target = await account('target');
const opposite = await account('opposite');
for (const id of [hot, target, opposite]) {
  const funded = await transfer(source, id, '10.00');
  if (funded.body.status !== 'posted') throw new Error(`Funding transfer failed: ${JSON.stringify(funded)}`);
}
await new Promise(resolve => setTimeout(resolve, 1100));
const debits = await Promise.all(Array.from({ length: 20 }, (_, index) => transfer(hot, target, '1.00', undefined, index)));
if (debits.filter(item => item.body.status === 'posted').length !== 10 || debits.filter(item => item.body.status === 'rejected').length !== 10) throw new Error(`Unexpected competing debit outcomes: ${JSON.stringify(debits)}`);
await new Promise(resolve => setTimeout(resolve, 1100));
const directions = await Promise.all(Array.from({ length: 10 }, (_, index) => index % 2 ? transfer(target, opposite, '1.00', undefined, index) : transfer(opposite, target, '1.00', undefined, index)));
if (directions.some(item => item.body.status !== 'posted')) throw new Error(`Opposite transfers failed: ${JSON.stringify(directions)}`);
const key = `study-same-${randomUUID()}`;
const replays = await Promise.all([transfer(target, opposite, '1.00', key, 0), transfer(target, opposite, '1.00', key, 1)]);
if (!replays[0].body.transfer_id || replays[0].body.transfer_id !== replays[1].body.transfer_id) throw new Error(`Same-key commands did not converge: ${JSON.stringify(replays)}`);
const reconciliations = await Promise.all([command('/api/reconciliation/runs', {}, undefined, 0), command('/api/reconciliation/runs', {}, undefined, 1)]);
if (reconciliations.some(item => ![200, 201, 409].includes(item.status))) throw new Error(`Reconciliation requests failed: ${JSON.stringify(reconciliations)}`);
console.log(JSON.stringify({ captured_at: new Date().toISOString(), accounts: { hot, target, opposite }, accepted_debits: 10, expected_rejections: 10, opposite_transfers: 10, same_key_transfer: replays[0].body.transfer_id, requests }));
