export type ReconciliationCommandIntent = Readonly<{
  version: 1;
  tenantId: string;
  idempotencyKey: string;
  state: "review" | "unknown" | "running";
  runId?: string;
  submittedAt?: string;
}>;

const visibleASCII = /^[\x21-\x7e]{16,255}$/;
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function reconciliationCommandStorageKey(tenantId: string): string {
  return `ledgersync:reconciliation-command:v1:${encodeURIComponent(tenantId)}`;
}

export function newReconciliationIdempotencyKey(): string {
  return `reconcile-${crypto.randomUUID()}`;
}

export function parseReconciliationCommandIntent(raw: string | null, tenantId: string): ReconciliationCommandIntent | null {
  if (!raw || raw.length > 2_048) return null;
  try {
    const value = JSON.parse(raw) as Partial<ReconciliationCommandIntent>;
    if (value.version !== 1
      || value.tenantId !== tenantId
      || typeof value.idempotencyKey !== "string"
      || !visibleASCII.test(value.idempotencyKey)
      || !["review", "unknown", "running"].includes(String(value.state))
      || value.runId !== undefined && (typeof value.runId !== "string" || !uuid.test(value.runId))
      || value.submittedAt !== undefined && (typeof value.submittedAt !== "string" || Number.isNaN(Date.parse(value.submittedAt)))
      || value.state === "running" && !value.runId) return null;
    return value as ReconciliationCommandIntent;
  } catch {
    return null;
  }
}

