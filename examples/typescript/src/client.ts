import { createClient, operations, type ClientOptions } from '../../../contracts/generated/typescript/ledgersync.ts';
import { IntentStore, type Intent } from './intents.ts';
import { createHash } from 'node:crypto';

const allowed = new Set(['createAccount', 'requestFunding', 'postFundingEvent', 'createTransfer', 'runReconciliation', 'registerDeveloperWebhook']);
export class PreparedClient {
  readonly store: IntentStore;
  readonly principals: Record<string, ClientOptions>;
  constructor(store: IntentStore, principals: Record<string, ClientOptions>) { this.store = store; this.principals = principals; }
  prepare(intent: Intent) {
    const operation = operations.find(value => value.id === intent.operationId);
    if (!operation?.idempotent || !allowed.has(operation.id) || !this.principals[intent.principal] || this.principals[intent.principal].baseUrl !== intent.baseUrl) throw new Error('Supported idempotent operation and configured principal required');
    const binding = this.principals[intent.principal];
    if (binding.actorAssertion !== undefined) throw new Error('The durable example requires an own-principal credential, not actor delegation');
    return this.store.prepare({ ...intent, credentialBinding: createHash('sha256').update(binding.token).digest('hex') });
  }
  async send(key: string) {
    const saved = this.store.get(key), binding = this.principals[saved.intent.principal];
    if (!binding || binding.baseUrl !== saved.intent.baseUrl || binding.actorAssertion !== undefined || saved.intent.credentialBinding !== createHash('sha256').update(binding.token).digest('hex')) throw new Error('Original credential, principal and destination binding required; inspect outcome before any rebind');
    const operation = operations.find(value => value.id === saved.intent.operationId);
    if (!operation?.idempotent || !allowed.has(operation.id)) throw new Error('Operation requires explicit outcome inspection, not blind retry');
    try {
      const response = await createClient(binding).call(operation, saved.intent.path, { headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key }, body: saved.bodyJSON });
      const reader = response.body?.getReader(); let text = ''; let bytes = 0; const decoder = new TextDecoder();
      if (reader) for (;;) { const chunk = await reader.read(); if (chunk.done) break; bytes += chunk.value.length; if (bytes > 65_536) { await reader.cancel(); throw new Error('Response exceeds example bound'); } text += decoder.decode(chunk.value, { stream: true }); }
      text += decoder.decode(); const body: unknown = text ? JSON.parse(text) : null;
      this.store.record(key, 'responded', response.status, text);
      const retryAfter = response.headers.get('Retry-After');
      return { kind: 'response' as const, status: response.status, body, replayed: response.headers.get('Idempotent-Replay') === 'true', retryAfterSeconds: retryAfter && /^\d{1,3}$/.test(retryAfter) ? Math.min(300, Number(retryAfter)) : null };
    } catch {
      this.store.record(key, 'unknown', null, null);
      return { kind: 'unknown' as const, instruction: 'Inspect or resend this exact persisted intent and key. No automatic retry was made.' };
    }
  }
}
