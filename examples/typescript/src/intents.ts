import { createHash } from 'node:crypto';
import type { DatabaseSync } from 'node:sqlite';
import { openState } from './storage.ts';

export type Intent = { key: string; principal: string; baseUrl: string; operationId: string; path: string; body: unknown; credentialBinding?: string };
export type SavedIntent = { intent: Intent; bodyJSON: string; state: string; responseStatus: number | null; responseJSON: string | null };

function canonical(value: unknown): string {
  if (value === null || typeof value === 'string' || typeof value === 'boolean') return JSON.stringify(value);
  if (Array.isArray(value)) return '[' + value.map(canonical).join(',') + ']';
  if (value && Object.getPrototypeOf(value) === Object.prototype) return '{' + Object.keys(value).sort().map(key => JSON.stringify(key) + ':' + canonical((value as Record<string, unknown>)[key])).join(',') + '}';
  throw new Error('Use explicit JSON strings for exact amounts and versions; unsupported intent value');
}

export class IntentStore {
  database: DatabaseSync;
  constructor(path: string) {
    this.database = openState(path);
    this.database.exec(`CREATE TABLE IF NOT EXISTS intents(key TEXT PRIMARY KEY, digest TEXT NOT NULL, principal TEXT NOT NULL, credential_binding TEXT NOT NULL, base_url TEXT NOT NULL, operation_id TEXT NOT NULL, path TEXT NOT NULL, body_json TEXT NOT NULL, state TEXT NOT NULL CHECK(state IN ('prepared','unknown','responded')), response_status INTEGER, response_json TEXT);`);
    if (!this.database.prepare('PRAGMA table_info(intents)').all().some(row => row.name === 'credential_binding')) { this.database.close(); throw new Error('Preserve this older example state and inspect its schema before reuse'); }
  }
  prepare(intent: Intent): SavedIntent {
    if (!/^[\x21-\x7e]{16,255}$/.test(intent.key) || !/^[a-z][a-z0-9-]{0,31}$/.test(intent.principal)) throw new Error('Bounded stable key and principal alias required');
    const bodyJSON = canonical(intent.body);
    if (Buffer.byteLength(bodyJSON) > 16_384) throw new Error('Example intent exceeds 16 KiB');
    const digest = createHash('sha256').update(canonical({ ...intent, credentialBinding: intent.credentialBinding ?? '', body: JSON.parse(bodyJSON) })).digest('hex');
    this.database.exec('BEGIN IMMEDIATE');
    try {
      const prior = this.database.prepare('SELECT digest FROM intents WHERE key=?').get(intent.key);
      if (prior && prior.digest !== digest) throw new Error('Changed intent requires a separate deliberate command; original key is preserved');
      if (!prior) {
        const count = this.database.prepare('SELECT count(*) AS n FROM intents').get()!;
        if (Number(count.n) >= 10_000) throw new Error('Example state capacity reached; preserve and archive before continuing');
        this.database.prepare("INSERT INTO intents(key,digest,principal,credential_binding,base_url,operation_id,path,body_json,state) VALUES(?,?,?,?,?,?,?,?,'prepared')").run(intent.key, digest, intent.principal, intent.credentialBinding ?? '', intent.baseUrl, intent.operationId, intent.path, bodyJSON);
      }
      this.database.exec('COMMIT');
    } catch (error) { this.database.exec('ROLLBACK'); throw error; }
    return this.get(intent.key);
  }
  get(key: string): SavedIntent {
    const row = this.database.prepare('SELECT * FROM intents WHERE key=?').get(key);
    if (!row) throw new Error('Original persisted intent not found');
    const intent = { key: String(row.key), principal: String(row.principal), credentialBinding: String(row.credential_binding), baseUrl: String(row.base_url), operationId: String(row.operation_id), path: String(row.path), body: JSON.parse(String(row.body_json)) };
    if (createHash('sha256').update(canonical(intent)).digest('hex') !== row.digest) throw new Error('Saved intent integrity check failed; no network dispatch is permitted');
    return { intent, bodyJSON: String(row.body_json), state: String(row.state), responseStatus: row.response_status === null ? null : Number(row.response_status), responseJSON: row.response_json === null ? null : String(row.response_json) };
  }
  record(key: string, state: 'unknown' | 'responded', status: number | null, json: string | null) {
    this.database.prepare('UPDATE intents SET state=?,response_status=?,response_json=? WHERE key=?').run(state, status, json, key);
  }
  close() { this.database.close(); }
}
