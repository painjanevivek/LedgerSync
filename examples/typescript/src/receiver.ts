import { createHmac, createHash, timingSafeEqual } from 'node:crypto';
import { createServer } from 'node:http';
import type { DatabaseSync } from 'node:sqlite';
import { openState } from './storage.ts';
import { receiverTrace, type ReceiverTrace } from './diagnostic.ts';

const uuid = /^[a-f0-9]{8}-[a-f0-9]{4}-[1-5][a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/;
export type Receipt = { status: number; outcome: string; headers?: Record<string, string> };
export class ReceiverStore {
  database: DatabaseSync;
  readonly keys: Readonly<Record<string, Buffer>>;
  constructor(path: string, keys: Readonly<Record<string, Buffer>>) {
    if (!Object.keys(keys).length || Object.entries(keys).some(([id, value]) => !/^[A-Za-z0-9][A-Za-z0-9._-]{2,99}$/.test(id) || value.length < 32)) throw new Error('Configured protected signing keys required');
    this.keys = keys; this.database = openState(path);
    this.database.exec(`CREATE TABLE IF NOT EXISTS events(event_id TEXT PRIMARY KEY,digest TEXT NOT NULL,event_type TEXT NOT NULL,aggregate_key TEXT NOT NULL,version TEXT,effect_applied INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS deliveries(delivery_id TEXT PRIMARY KEY,event_id TEXT NOT NULL,digest TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS projections(aggregate_key TEXT PRIMARY KEY,version TEXT NOT NULL,event_id TEXT NOT NULL);`);
  }
  accept(raw: Buffer, headers: Record<string, string>, now = Date.now()): Receipt {
    if (raw.length > 262_144) return { status: 413, outcome: 'body_too_large' };
    const keyID = headers['x-ledgersync-key-id'], key = Object.hasOwn(this.keys, keyID) ? this.keys[keyID] : undefined;
    const delivery = headers['x-ledgersync-delivery-id'], verification = headers['x-ledgersync-verification-id'];
    const id = delivery || verification, stamp = headers['x-ledgersync-timestamp'], signature = headers['x-ledgersync-signature'];
    if (!key || !id || !uuid.test(id) || Boolean(delivery) === Boolean(verification) || !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(stamp ?? '') || !Number.isFinite(Date.parse(stamp)) || Math.abs(now - Date.parse(stamp)) > 300_000 || !/^v1=[a-f0-9]{64}$/.test(signature ?? '')) return { status: 401, outcome: 'invalid_signature_context' };
    const mac = createHmac('sha256', key).update(`${stamp}.${id}.`).update(raw).update(`.${keyID}`).digest();
    if (!timingSafeEqual(mac, Buffer.from(signature.slice(3), 'hex'))) return { status: 401, outcome: 'invalid_signature' };
    let value: Record<string, unknown>; try { value = JSON.parse(raw.toString('utf8')); } catch { return { status: 400, outcome: 'invalid_json' }; }
    if (!value || typeof value !== 'object' || Array.isArray(value)) return { status: 400, outcome: 'invalid_envelope' };
    if (verification) {
      if (typeof value.challenge !== 'string' || value.challenge.length < 32 || value.challenge.length > 255 || typeof value.expires_at !== 'string' || !Number.isFinite(Date.parse(value.expires_at)) || Date.parse(value.expires_at) <= now) return { status: 400, outcome: 'expired_challenge' };
      return { status: 204, outcome: 'verification', headers: { 'X-LedgerSync-Verification': 'v1=' + createHmac('sha256', key).update(value.challenge).digest('hex') } };
    }
    if (typeof value.event_id !== 'string' || !uuid.test(value.event_id) || value.event_id !== headers['x-ledgersync-event-id'] || value.event_type !== headers['x-ledgersync-event-type']) return { status: 400, outcome: 'signed_event_identity_mismatch' };
    let aggregate: string, version: string | null = null;
    if (value.event_type === 'transfer.posted' && typeof value.transfer_id === 'string' && uuid.test(value.transfer_id)) { aggregate = 'transfer:' + value.transfer_id; version = '1'; }
    else if (value.event_type === 'account.balance.changed.v1' && typeof value.account_id === 'string' && uuid.test(value.account_id) && typeof value.balance_version === 'string' && /^[1-9][0-9]{0,18}$/.test(value.balance_version) && BigInt(value.balance_version) <= 9_223_372_036_854_775_807n) { aggregate = 'balance:' + value.account_id; version = value.balance_version; }
    else return { status: 400, outcome: 'unsupported_event' };
    const digest = createHash('sha256').update(raw).digest('hex');
    this.database.exec('BEGIN IMMEDIATE');
    try {
      const priorDelivery = this.database.prepare('SELECT event_id,digest FROM deliveries WHERE delivery_id=?').get(id);
      const prior = this.database.prepare('SELECT digest FROM events WHERE event_id=?').get(value.event_id);
      if (priorDelivery && (priorDelivery.event_id !== value.event_id || priorDelivery.digest !== digest) || prior && prior.digest !== digest) { this.database.exec('ROLLBACK'); return { status: 409, outcome: 'immutable_event_conflict' }; }
      if (!priorDelivery && Number(this.database.prepare('SELECT count(*) AS n FROM deliveries').get()!.n) >= 20_000) { this.database.exec('ROLLBACK'); return { status: 503, outcome: 'example_capacity_reached' }; }
      if (prior) { this.database.prepare('INSERT OR IGNORE INTO deliveries VALUES(?,?,?)').run(id, value.event_id, digest); this.database.exec('COMMIT'); return { status: 204, outcome: 'duplicate' }; }
      if (Number(this.database.prepare('SELECT count(*) AS n FROM events').get()!.n) >= 10_000) { this.database.exec('ROLLBACK'); return { status: 503, outcome: 'example_capacity_reached' }; }
      const current = version ? this.database.prepare('SELECT version FROM projections WHERE aggregate_key=?').get(aggregate) : null;
      if (version && current && BigInt(version) === BigInt(String(current.version))) { this.database.exec('ROLLBACK'); return { status: 409, outcome: 'aggregate_version_conflict' }; }
      const applied = !version || !current || BigInt(version) > BigInt(String(current.version));
      this.database.prepare('INSERT INTO events VALUES(?,?,?,?,?,?)').run(value.event_id, digest, String(value.event_type), aggregate, version, applied ? 1 : 0);
      this.database.prepare('INSERT INTO deliveries VALUES(?,?,?)').run(id, value.event_id, digest);
      if (version && applied) this.database.prepare('INSERT INTO projections VALUES(?,?,?) ON CONFLICT(aggregate_key) DO UPDATE SET version=excluded.version,event_id=excluded.event_id').run(aggregate, version, value.event_id);
      this.database.exec('COMMIT'); return { status: 204, outcome: applied ? 'applied' : 'stale' };
    } catch { this.database.exec('ROLLBACK'); return { status: 503, outcome: 'state_unavailable' }; }
  }
  close() { this.database.close(); }
}

export function receiverServer(store: ReceiverStore, shouldFail: () => boolean = () => false, observeTrace?: (span: ReceiverTrace) => void) {
  return createServer(async (request, response) => {
    if (observeTrace) response.once('finish', () => {
      const span = receiverTrace(request.headers.traceparent, response.statusCode);
      if (span) {
        try { observeTrace(span); } catch { /* Diagnostic loss cannot change signed delivery handling. */ }
      }
    });
    if (request.method !== 'POST' || request.url !== '/hooks') { response.writeHead(404).end(); return; }
    const headers: Record<string, string> = {};
    for (let i = 0; i < request.rawHeaders.length; i += 2) {
      const key = request.rawHeaders[i].toLowerCase();
      if (key.startsWith('x-ledgersync-') && headers[key] !== undefined) { response.writeHead(400).end(); return; }
      headers[key] = request.rawHeaders[i + 1];
    }
    const chunks: Buffer[] = []; let size = 0;
    try {
      for await (const chunk of request) { size += chunk.length; if (size > 262_144) { response.writeHead(413).end(); return; } chunks.push(Buffer.from(chunk)); }
      if (shouldFail()) { response.writeHead(503, { 'Retry-After': '1' }).end(); return; }
      const result = store.accept(Buffer.concat(chunks), headers);
      response.writeHead(result.status, { ...result.headers, 'Cache-Control': 'no-store' }).end();
    } catch { if (!response.headersSent) response.writeHead(503).end(); }
  });
}
