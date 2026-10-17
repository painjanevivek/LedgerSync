import { performance } from 'node:perf_hooks';
import { setTimeout as sleep } from 'node:timers/promises';
import { chooseIntent, accountIDs } from './definition.mjs';
const forbiddenPorts = new Set([3000, 3001, 3100, 3102, 3103, 3117, 5432, 6379, 8080, 4317, 4318]);
export function validateBFFOrigin(value) {
    const u = new URL(value);
    if (u.protocol !== 'http:' || u.hostname !== '127.0.0.1' || !/^\d+$/.test(u.port) || Number(u.port) < 1024 || Number(u.port) > 65535 || forbiddenPorts.has(Number(u.port)) || u.username || u.password || u.pathname !== '/' || u.search || u.hash)
        throw Error('An owned dynamic loopback BFF origin is required');
    return u.origin;
}
async function boundedJSON(response) {
    if (!response.body)
        throw Error('HTTP response body is absent');
    const reader = response.body.getReader();
    let size = 0;
    const parts = [];
    try {
        for (;;) {
            const { done, value } = await reader.read();
            if (done)
                break;
            size += value.byteLength;
            if (size > 65536)
                throw Error('HTTP response exceeded the evidence bound');
            parts.push(Buffer.from(value));
        }
    }
    finally {
        await reader.cancel().catch(() => {
        });
    }
    return JSON.parse(Buffer.concat(parts).toString('utf8'));
}
export class CapacityHTTPClient {
    constructor(origin, { fetcher = fetch, signal, timeout = 12000 } = {}) {
        this.origin = validateBFFOrigin(origin);
        this.fetcher = fetcher;
        this.signal = signal;
        this.timeout = timeout;
        this.cookies = new Map();
        this.csrf = '';
        this.requests = 0;
        this.httpFailures = 0;
        this.transportFailures = 0;
    }
    async request(path, { method = 'GET', body, intent, discard = false } = {}) {
        if (!path.startsWith('/api/') || path.includes('://') || path.includes('..'))
            throw Error('Capacity requests must stay on the owned BFF');
        this.signal?.throwIfAborted();
        this.requests++;
        const signal = this.signal ? AbortSignal.any([this.signal, AbortSignal.timeout(this.timeout)]) : AbortSignal.timeout(this.timeout);
        const headers = {
            Cookie: [...this.cookies].map(([k, v]) => `${k}=${v}`).join('; '),
            Origin: this.origin,
            'Content-Type': 'application/json',
            'X-CSRF-Token': this.csrf
        };
        if (intent) {
            headers['Idempotency-Key'] = intent.key;
            if (intent.reference)
                headers['X-LedgerSync-Request-Reference'] = intent.reference;
        }
        let response;
        try {
            response = await this.fetcher(this.origin + path, {
                method,
                headers,
                body: body === undefined ? undefined : JSON.stringify(body),
                redirect: 'manual',
                signal
            });
        }
        catch (error) {
            this.transportFailures++;
            throw error;
        }
        if (response.status >= 400)
            this.httpFailures++;
        for (const raw of response.headers.getSetCookie()) {
            const first = raw.split(';', 1)[0], at = first.indexOf('=');
            if (at > 0)
                this.cookies.set(first.slice(0, at), first.slice(at + 1));
        }
        if (this.cookies.size > 8)
            throw Error('Unexpected BFF cookie growth');
        if (discard) {
            await response.body?.cancel();
            return {
                status: response.status,
                discarded: true
            };
        }
        if (response.status >= 300 && response.status < 400) {
            await response.body?.cancel();
            return {
                status: response.status,
                location: response.headers.get('location')
            };
        }
        try {
            return {
                status: response.status,
                body: await boundedJSON(response),
                replayed: response.headers.get('idempotent-replay') === 'true'
            };
        }
        catch (error) {
            this.transportFailures++;
            throw error;
        }
    }
    async authenticate() {
        const signIn = await this.request('/api/auth/sign-in?return_to=/');
        if (![302, 303, 307].includes(signIn.status) || !signIn.location || new URL(signIn.location, this.origin).origin !== this.origin || this.cookies.size === 0)
            throw Error('Owned BFF sign-in did not establish a local session');
        const session = await this.request('/api/session');
        if (session.status !== 200 || typeof session.body.csrf_token !== 'string' || !session.body.csrf_token || session.body.csrf_token.length > 256)
            throw Error('Authenticated CSRF session required');
        this.csrf = session.body.csrf_token;
    }
    async prepare(intent) {
        const saved = await this.request('/api/transfers/recovery', {
            method: 'POST',
            body: intent.body,
            intent
        });
        if (![200, 201].includes(saved.status) || saved.body.request_reference !== intent.reference || !['saved', 'committed'].includes(saved.body.state))
            throw Error('Original transfer intent was not durably saved');
    }
    async submit(intent) {
        await this.prepare(intent);
        let result;
        for (let attempt = 0; attempt < 3; attempt++) {
            try {
                result = await this.request('/api/transfers', {
                    method: 'POST',
                    body: intent.body,
                    intent
                });
                if (result.status === 201 && typeof result.body.transfer_id === 'string' && /^[a-f0-9-]{36}$/.test(result.body.transfer_id) && ['posted', 'rejected'].includes(result.body.status))
                    return result;
                if (![429, 502, 503, 504].includes(result.status) && result.body.error?.code !== 'request_in_progress' && result.body.error_code !== 'request_in_progress')
                    break;
            }
            catch (error) {
                this.signal?.throwIfAborted();
                if (attempt === 2)
                    throw error;
            }
            await sleep(100 * (attempt + 1), undefined, {
                signal: this.signal
            });
        }
        throw Error(`Unresolved original-key transfer outcome (${result?.status ?? 'transport'})`);
    }
}
export async function runOfferedLoad({ rate, seconds, execute, concurrency = 8, pendingLimit = 32, signal, stopRequested = () => false, now = () => performance.now(), wait = ms => sleep(ms, undefined, {
    signal
}) }) {
    if (![5, 10, 20].includes(rate) || !Number.isInteger(seconds) || seconds < 1 || seconds > 300 || concurrency !== 8 || pendingLimit !== 32)
        throw Error('Fixed bounded offered-load method required');
    const start = now(), queue = [], active = new Set(), results = [];
    let dropped = 0, maxLag = 0, fatal, offered = 0, stopConstraint;
    const dispatch = () => {
        while (!fatal && active.size < concurrency && queue.length) {
            const item = queue.shift();
            const begun = now();
            const observation = () => ({
                index: item.index,
                latency_ms: now() - item.due,
                service_ms: now() - begun,
                queue_wait_ms: begun - item.due
            });
            const task = Promise.resolve().then(() => execute(item.index)).then(value => results.push({
                ...value,
                ...observation()
            }), error => {
                if (error.code === 'FINANCIAL_INVARIANT') {
                    fatal = error;
                    queue.length = 0;
                }
                results.push({
                    ...observation(),
                    technical_failure: true
                });
            }).finally(() => {
                active.delete(task);
                dispatch();
            });
            active.add(task);
        }
    };
    let offeredEnd;
    try {
        for (let index = 0; index < rate * seconds; index++) {
            if (fatal)
                throw fatal;
            stopConstraint = stopRequested();
            if (stopConstraint)
                break;
            signal?.throwIfAborted();
            const due = start + index * 1000 / rate;
            if (now() < due)
                await wait(due - now());
            maxLag = Math.max(maxLag, now() - due);
            offered++;
            if (queue.length >= pendingLimit) {
                dropped++;
                continue;
            }
            queue.push({
                index,
                due
            });
            dispatch();
        }
        const end = start + seconds * 1000;
        if (!stopConstraint && now() < end)
            await wait(end - now());
        offeredEnd = now();
    }
    catch (error) {
        fatal = error;
        queue.length = 0;
    }
    finally {
        while (active.size || queue.length) {
            dispatch();
            await Promise.all([...active]);
        }
    }
    if (fatal)
        throw fatal;
    return {
        offered,
        requested_offers: rate * seconds,
        stop_constraint: stopConstraint || null,
        completed: results.length,
        dropped,
        generator_max_lag_ms: maxLag,
        observed_seconds: (offeredEnd - start) / 1000,
        drain_seconds: (now() - offeredEnd) / 1000,
        results
    };
}
export function percentiles(samples) {
    if (!samples.length || samples.some(n => !Number.isFinite(n) || n < 0))
        throw Error('Complete finite latency samples required');
    const values = [...samples].sort((a, b) => a - b), at = p => values[Math.min(values.length - 1, Math.ceil(values.length * p) - 1)];
    return {
        p50_ms: at(.5),
        p95_ms: at(.95),
        p99_ms: at(.99)
    };
}
export class CapacityWorkload {
    constructor(origins, profile, runID, { signal } = {}) {
        this.profile = profile;
        this.runID = runID;
        this.signal = signal;
        this.sequence = 0;
        this.receipts = new Map();
        this.unresolved = new Map();
        this.clients = Array.from({
            length: 8
        }, (_, i) => new CapacityHTTPClient(origins[i % 2], {
            signal
        }));
        this.stages = [];
    }
    async authenticate() {
        for (const client of this.clients)
            await client.authenticate();
    }
    clientFor(intent, index = 0) {
        return this.clients[(index % 4) * 2 + intent.tenant];
    }
    observe(intent, result) {
        const expected = chooseIntent(this.profile, intent.original, this.runID).kind === 'expected_rejection' ? 'rejected' : 'posted';
        const invariant = message => Object.assign(Error(message), {
            code: 'FINANCIAL_INVARIANT'
        });
        if (result.body.status !== expected)
            throw invariant('Business result differs from the independent workload intent');
        if (result.body.currency !== 'INR' || result.body.amount_minor !== intent.body.amount.minorUnits)
            throw invariant('Receipt amount differs from the independent exact-money intent');
        const previous = this.receipts.get(intent.key);
        if (previous && (previous.id !== result.body.transfer_id || previous.status !== result.body.status))
            throw invariant('One original key returned different financial receipts');
        this.receipts.set(intent.key, {
            id: result.body.transfer_id,
            status: result.body.status,
            intent: structuredClone(intent.body)
        });
        this.unresolved.delete(intent.key);
        return {
            kind: intent.kind,
            status: result.body.status,
            replayed: result.replayed,
            key: intent.key
        };
    }
    async stage(name, rate, seconds, { stopRequested } = {}) {
        const startedAt=new Date().toISOString();
        const offset = this.sequence;
        this.sequence += rate * seconds;
        if (this.sequence > 6000)
            throw Error('Capacity operation ceiling reached');
        const result = await runOfferedLoad({
            rate,
            seconds,
            stopRequested,
            signal: this.signal,
            execute: async (index) => {
                const intent = chooseIntent(this.profile, offset + index, this.runID), client = this.clientFor(intent, index);
                this.unresolved.set(intent.key, intent);
                const observation = this.observe(intent, await client.submit(intent));
                if (index % 5 === 4) {
                    const read = await client.request(`/api/accounts/${intent.body.destinationAccountId}/balance`);
                    if (read.status !== 200)
                        throw Error('Mixed balance read failed');
                    observation.balance_reads = 1;
                }
                if (index % 50 === 49) {
                    const read = await client.request('/api/reconciliation/runs?limit=10');
                    if (read.status !== 200)
                        throw Error('Mixed reconciliation read failed');
                    observation.reconciliation_reads = 1;
                }
                return observation;
            }
        });
        this.stages.push({
            name,
            rate,
            seconds,
            started_at:startedAt,
            completed_at:new Date().toISOString(),
            ...result
        });
        return result;
    }
    async recoverOriginals() {
        for (const intent of [...this.unresolved.values()])
            this.observe(intent, await this.clientFor(intent).submit(intent));
    }
    async probeIsolation() {
        for (const tenant of [0, 1]) {
            const foreign = chooseIntent('multi-tenant', tenant === 0 ? 1 : 0, this.runID).body.sourceAccountId;
            const result = await this.clients[tenant].request(`/api/accounts/${foreign}`);
            if (result.status !== 404)
                throw Error('Tenant isolation probe did not deny the foreign account');
        }
        return {
            tenant_leaks: 0,
            probes: 2
        };
    }
    async loseBody() {
        const intent = chooseIntent(this.profile, this.sequence++, this.runID);
        await this.clientFor(intent).prepare(intent);
        const result = await this.clientFor(intent).request('/api/transfers', {
            method: 'POST',
            body: intent.body,
            intent,
            discard: true
        });
        if (result.status !== 201)
            throw Error('Lost-body recovery requires a committed response header');
        this.unresolved.set(intent.key, intent);
        return intent;
    }
    async recoverLost(intent) {
        const result = await this.clientFor(intent).submit(intent);
        if (!result.replayed)
            throw Error('Lost-body original-key recovery did not replay');
        return this.observe(intent, result);
    }
    counts() {
        const balances = new Map(accountIDs.map((id, i) => [id, i === 4 || i === 7 ? 0n : 1000000n]));
        const versions = new Map(accountIDs.map(id => [id, 0n]));
        for (const receipt of this.receipts.values())
            if (receipt.status === 'posted') {
                const amount = BigInt(receipt.intent.amount.minorUnits), from = receipt.intent.sourceAccountId, to = receipt.intent.destinationAccountId;
                balances.set(from, balances.get(from) - amount);
                balances.set(to, balances.get(to) + amount);
                versions.set(from, versions.get(from) + 1n);
                versions.set(to, versions.get(to) + 1n);
            }
        return {
            posted: [...this.receipts.values()].filter(r => r.status === 'posted').length,
            rejected: [...this.receipts.values()].filter(r => r.status === 'rejected').length,
            unresolved: this.unresolved.size,
            http_requests: this.clients.reduce((sum, c) => sum + c.requests, 0),
            expected_balances: [...balances].map(([account_id, minor]) => ({
                account_id,
                ledger_minor: String(minor),
                available_minor: String(minor),
                balance_version: String(versions.get(account_id))
            }))
        };
    }
}
