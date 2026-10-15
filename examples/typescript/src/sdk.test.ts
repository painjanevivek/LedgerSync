import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { createClient, operations } from '../../../contracts/generated/typescript/ledgersync.ts';

test('generated client preserves /api over real HTTP and refreshes delegated context', async () => {
  const seen: Array<{ path: string; actor: string | undefined; auth: string | undefined }> = [];
  const server = createServer((req, res) => { seen.push({ path: req.url!, actor: req.headers['x-ledgersync-actor-assertion'] as string | undefined, auth: req.headers.authorization }); res.end('{}'); });
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  const address = server.address(); assert.ok(address && typeof address !== 'string');
  try {
    const baseUrl = `http://127.0.0.1:${address.port}/api`, operation = operations.find(o => o.id === 'listOwnedAccounts')!; assert.ok(operation);
    let calls = 0;
    const delegated = createClient({ baseUrl, token: 'synthetic-token', actorAssertion: () => `fresh-${++calls}` });
    await delegated.call(operation, '/me/accounts?limit=1', { headers: new Headers({ Authorization: 'must-not-override' }) });
    await delegated.call(operation, '/me/accounts');
    await createClient({ baseUrl, token: 'own-principal' }).call(operation, '/me/accounts', { headers: { 'X-LedgerSync-Actor-Assertion': 'must-be-removed' } });
    assert.deepEqual(seen.map(s => s.path), ['/api/me/accounts?limit=1', '/api/me/accounts', '/api/me/accounts']);
    assert.deepEqual(seen.map(s => s.actor), ['fresh-1', 'fresh-2', undefined]);
    assert.deepEqual(seen.map(s => s.auth), ['Bearer synthetic-token', 'Bearer synthetic-token', 'Bearer own-principal']);
    for (const path of ['//outside.test', '/https://outside.test', '/../outside', '/%2e%2e/outside', '/me/accounts#fragment', '/\\outside.test']) await assert.rejects(delegated.call(operation, path));
    assert.equal(seen.length, 3);
  } finally { server.closeAllConnections(); await new Promise<void>(resolve => server.close(() => resolve())); }
});
