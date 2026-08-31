export type FinancialPostDomain = "funding" | "correction";

export type FinancialPostIntent = Readonly<{
  version: 1;
  domain: FinancialPostDomain;
  tenantId: string;
  recordId: string;
  idempotencyKey: string;
  state: "unknown";
}>;

const visibleASCII = /^[\x21-\x7e]{16,255}$/;
const exactKeys = ["version", "domain", "tenantId", "recordId", "idempotencyKey", "state"] as const;

function boundedIdentity(value: unknown): value is string {
  return typeof value === "string" && value.length >= 1 && value.length <= 128;
}

export function financialPostStorageKey(
  domain: FinancialPostDomain,
  tenantId: string,
  recordId: string,
): string {
  return `ledgersync:financial-post:v1:${domain}:${encodeURIComponent(tenantId)}:${encodeURIComponent(recordId)}`;
}

export function createFinancialPostIntent(
  domain: FinancialPostDomain,
  tenantId: string,
  recordId: string,
  idempotencyKey = `${domain}-post-${crypto.randomUUID()}`,
): FinancialPostIntent {
  return { version: 1, domain, tenantId, recordId, idempotencyKey, state: "unknown" };
}

export function parseFinancialPostIntent(
  raw: string | null,
  domain: FinancialPostDomain,
  tenantId: string,
  recordId: string,
): FinancialPostIntent | null {
  if (!raw || raw.length > 2_048) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (!value || Array.isArray(value) || typeof value !== "object") return null;
    const record = value as Partial<FinancialPostIntent> & Record<string, unknown>;
    if (Object.keys(record).some((key) => !exactKeys.includes(key as (typeof exactKeys)[number]))) return null;
    return record.version === 1
      && record.domain === domain
      && record.tenantId === tenantId
      && record.recordId === recordId
      && boundedIdentity(record.tenantId)
      && boundedIdentity(record.recordId)
      && typeof record.idempotencyKey === "string"
      && visibleASCII.test(record.idempotencyKey)
      && record.state === "unknown"
      ? record as FinancialPostIntent
      : null;
  } catch {
    return null;
  }
}
