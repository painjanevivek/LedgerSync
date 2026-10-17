import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { CapacityHTTPClient, runOfferedLoad, percentiles, validateBFFOrigin } from './http-capacity/driver.mjs';
import { chooseIntent } from './http-capacity/definition.mjs';
import { createAPIRouter } from './http-capacity/router.mjs';
async function listen(t, server) {
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    t.after(() => new Promise(resolve => {
        server.closeAllConnections();
        server.close(resolve);
    }));
    return `http://127.0.0.1:${server.address().port}`;
}
const json = (res, status, value, headers = {}) => res.writeHead(status, {
    'Content-Type': 'application/json',
    ...headers
}).end(JSON.stringify(value));
test('actual HTTP response loss retries the original saved intent with one financial receipt', async (t) => {
    const intents = [], committed = new Map();
    let writes = 0;
    const origin = await listen(t, http.createServer(async (req, res) => {
        if (req.url.startsWith('/api/auth/sign-in')) {
            res.writeHead(303, {
                'Set-Cookie': 'capacity=opaque; HttpOnly; SameSite=Lax',
                Location: '/'
            }).end();
            return;
        }
        assert.equal(req.headers.cookie, 'capacity=opaque');
        if (req.url === '/api/session') {
            json(res, 200, {
                csrf_token: 'synthetic-csrf'
            });
            return;
        }
        assert.equal(req.headers['x-csrf-token'], 'synthetic-csrf');
        assert.equal(req.headers.origin, origin);
        let body = '';
        for await (const part of req)
            body += part;
        const intent = {
            key: req.headers['idempotency-key'],
            reference: req.headers['x-ledgersync-request-reference'],
            body: JSON.parse(body)
        };
        intents.push(intent);
        if (req.url === '/api/transfers/recovery') {
            json(res, 201, {
                state: 'saved',
                request_reference: intent.reference
            });
            return;
        }
        const prior = committed.get(intent.key);
        if (prior) {
            json(res, 201, prior, {
                'Idempotent-Replay': 'true'
            });
            return;
        }
        const receipt = {
            transfer_id: '00000000-0000-4000-8000-000000009399',
            status: 'posted'
        };
        committed.set(intent.key, receipt);
        writes++;
        req.socket.destroy();
    }));
    const client = new CapacityHTTPClient(origin);
    await client.authenticate();
    const intent = chooseIntent('multi-tenant', 1, '1234abcd'), result = await client.submit(intent);
    assert.equal(result.replayed, true);
    assert.equal(writes, 1);
    assert.equal(client.transportFailures, 1);
    assert.equal(intents.length, 3);
    for (const captured of intents)
        assert.deepEqual(captured, {
            key: intent.key,
            reference: intent.reference,
            body: intent.body
        });
});
test('truncated response bodies count as transport failures and redirects never reach another origin', async (t) => {
    let external = 0;
    const other = await listen(t, http.createServer((_req, res) => {
        external++;
        res.end();
    }));
    const origin = await listen(t, http.createServer((req, res) => {
        if (req.url.includes('sign-in')) {
            res.writeHead(303, {
                Location: other,
                'Set-Cookie': 'capacity=opaque'
            }).end();
            return;
        }
        res.writeHead(200, {
            'Content-Type': 'application/json',
            'Content-Length': '100'
        });
        res.write('{');
        res.flushHeaders();
        setImmediate(() => res.destroy());
    }));
    const client = new CapacityHTTPClient(origin);
    await assert.rejects(() => client.authenticate(), /local session/);
    assert.equal(external, 0);
    await assert.rejects(() => client.request('/api/session'));
    assert.equal(client.transportFailures, 1);
    for (const value of ['http://localhost:45555', 'http://127.0.0.1:3000', 'https://example.com', 'http://127.0.0.1:45555/other'])
        assert.throws(() => validateBFFOrigin(value));
    await assert.rejects(() => client.request('/api/../escape'));
});
test('router alternates owned backends and never retries an uncertain write', async (t) => {
    const calls = [0, 0];
    const first = await listen(t, http.createServer((req, res) => {
        calls[0]++;
        if (req.method === 'POST')
            req.socket.destroy();
        else
            json(res, 200, {
                backend: 0
            });
    }));
    const second = await listen(t, http.createServer((_req, res) => {
        calls[1]++;
        json(res, 200, {
            backend: 1
        });
    }));
    const router = await listen(t, createAPIRouter([first, second]));
    for (const backend of [0, 1, 0, 1])
        assert.equal((await (await fetch(router + '/readyz')).json()).backend, backend);
    const response = await fetch(router + '/api/transfers', {
        method: 'POST',
        body: '{}'
    });
    assert.equal(response.status, 503);
    await response.body.cancel();
    assert.deepEqual(calls, [3, 2]);
    assert.throws(() => createAPIRouter(['http://external.example:8080']));
});
test('offered load bounds callers and queued work; measured latency includes arrival queue time', async () => {
    let time = 0, release, active = 0, peak = 0;
    const gate = new Promise(resolve => release = resolve);
    const result = await runOfferedLoad({
        rate: 20,
        seconds: 5,
        now: () => time,
        wait: async (ms) => {
            time += ms;
            if (time >= 5000)
                release();
            await new Promise(setImmediate);
        },
        execute: async () => {
            active++;
            peak = Math.max(peak, active);
            await gate;
            active--;
            return {
                kind: 'new',
                status: 'posted'
            };
        }
    });
    assert.equal(peak, 8);
    assert.equal(result.completed, 40);
    assert.equal(result.dropped, 60);
    assert.equal(result.completed + result.dropped, result.offered);
    assert.ok(result.results.some(r => r.queue_wait_ms > 1000));
    assert.ok(percentiles(result.results.map(r => r.latency_ms)).p95_ms > 1000);
});
test('a financial mismatch stops new offers and drains all in-flight callers', async () => {
    let time = 0, active = 0, calls = 0;
    await assert.rejects(() => runOfferedLoad({
        rate: 5,
        seconds: 1,
        now: () => time,
        wait: async (ms) => {
            time += ms;
            await new Promise(setImmediate);
        },
        execute: async () => {
            calls++;
            active++;
            await Promise.resolve();
            active--;
            throw Object.assign(Error('wrong money'), {
                code: 'FINANCIAL_INVARIANT'
            });
        }
    }), /wrong money/);
    assert.equal(active, 0);
    assert.equal(calls, 1);
});
test('a sampled budget stop ends the offered interval without inventing five minutes of work', async () => {
    let time = 0;
    const result = await runOfferedLoad({
        rate: 10,
        seconds: 300,
        now: () => time,
        wait: async (ms) => {
            time += ms;
            await new Promise(setImmediate);
        },
        stopRequested: () => time >= 1000 ? 'queue_or_database_budget' : false,
        execute: async () => ({
            status: 'posted',
            kind: 'new'
        })
    });
    assert.ok(result.offered < 3000);
    assert.equal(result.requested_offers, 3000);
    assert.ok(result.observed_seconds < 300);
    assert.equal(result.offered, result.completed + result.dropped);
    assert.equal(result.stop_constraint, 'queue_or_database_budget');
});
