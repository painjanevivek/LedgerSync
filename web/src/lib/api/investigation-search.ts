export const investigationRecordTypes = [
  "account",
  "transfer",
  "funding",
  "event",
  "reconciliation_run",
  "reconciliation_mismatch",
  "correction",
  "request_reference",
] as const;

export type InvestigationRecordType = typeof investigationRecordTypes[number];

export type InvestigationSearchResult = Readonly<{
  record_type: InvestigationRecordType;
  record_id: string;
  related_record_type?: Exclude<InvestigationRecordType, "request_reference" | "reconciliation_mismatch">;
  related_record_id?: string;
  safe_label: string;
  status: string;
  occurred_at: string;
  source: "postgresql";
  freshness: "search_snapshot";
}>;

export type InvestigationSearchPage = Readonly<{
  results: InvestigationSearchResult[];
  query_kind: "immutable_id" | "approved_reference";
  generated_at: string;
  truncated: boolean;
}>;

export type SanitizedInvestigationSearchResponse = Readonly<{ status: number; body: Readonly<Record<string, unknown>> }>;

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/u;
const safeText = /^[A-Za-z0-9][A-Za-z0-9 ._:/()'-]{0,127}$/u;
const recordTypes = new Set<string>(investigationRecordTypes);
const relatedTypes = new Set(["account", "transfer", "funding", "event", "reconciliation_run", "correction"]);
const statuses = new Set([
  "active", "frozen", "closed", "pending", "posted", "rejected", "requested", "approved", "compensated",
  "published", "dead", "retrying", "matched", "mismatch", "failed", "cancelled", "expired", "allowed", "denied", "succeeded",
  "recorded",
]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function exactKeys(value: Record<string, unknown>, allowed: readonly string[]) {
  return Object.keys(value).every((key) => allowed.includes(key));
}

function timestamp(value: unknown): value is string {
  return typeof value === "string" && value.length <= 64 && !Number.isNaN(Date.parse(value));
}

function sanitizeResult(value: unknown): InvestigationSearchResult | null {
  if (!isRecord(value) || !exactKeys(value, ["record_type", "record_id", "related_record_type", "related_record_id", "safe_label", "status", "occurred_at", "source", "freshness"])) return null;
  if (typeof value.record_type !== "string" || !recordTypes.has(value.record_type)
    || typeof value.record_id !== "string" || !uuid.test(value.record_id)
    || typeof value.safe_label !== "string" || !safeText.test(value.safe_label)
    || typeof value.status !== "string" || !statuses.has(value.status)
    || !timestamp(value.occurred_at)
    || value.source !== "postgresql" || value.freshness !== "search_snapshot") return null;
  const hasRelatedType = value.related_record_type !== undefined;
  const hasRelatedID = value.related_record_id !== undefined;
  if (hasRelatedType !== hasRelatedID) return null;
  if (hasRelatedType && (typeof value.related_record_type !== "string" || !relatedTypes.has(value.related_record_type)
    || typeof value.related_record_id !== "string" || !uuid.test(value.related_record_id))) return null;
  return value as InvestigationSearchResult;
}

function sanitizeError(status: number, value: unknown): SanitizedInvestigationSearchResponse {
  const error = isRecord(value) && isRecord(value.error) ? value.error : undefined;
  const code = typeof error?.code === "string" ? error.code : "";
  if (status === 400 && (code === "invalid_request" || code === "validation_failed")) return { status, body: { error: { code } } };
  if (status === 401 && code === "unauthorized") return { status, body: { error: { code } } };
  if (status === 403 && code === "forbidden") return { status, body: { error: { code } } };
  if (status === 429 && code === "rate_limited") return { status, body: { error: { code } } };
  if (status === 503 && (code === "evidence_unavailable" || code === "temporary_unavailable")) return { status, body: { error: { code } } };
  if (status === 504 && code === "upstream_timeout") return { status, body: { error: { code } } };
  return { status: 503, body: { error: { code: "evidence_unavailable" } } };
}

export function sanitizeInvestigationSearch(status: number, value: unknown): SanitizedInvestigationSearchResponse {
  if (status < 200 || status >= 300) return sanitizeError(status, value);
  if (!isRecord(value) || !exactKeys(value, ["results", "query_kind", "generated_at", "truncated"])
    || !Array.isArray(value.results) || value.results.length > 20
    || value.query_kind !== "immutable_id" && value.query_kind !== "approved_reference"
    || !timestamp(value.generated_at) || typeof value.truncated !== "boolean") return sanitizeError(503, null);
  const results = value.results.map(sanitizeResult);
  if (results.some((result) => result === null)) return sanitizeError(503, null);
  return { status, body: { results, query_kind: value.query_kind, generated_at: value.generated_at, truncated: value.truncated } };
}

export function parseInvestigationSearchBody(status: number, body: string): SanitizedInvestigationSearchResponse {
  if (new TextEncoder().encode(body).byteLength > 65_536) return sanitizeError(503, null);
  try { return sanitizeInvestigationSearch(status, JSON.parse(body) as unknown); }
  catch { return sanitizeError(503, null); }
}

export async function readBoundedInvestigationSearchBody(response: Response): Promise<string> {
  const declaredLength = response.headers.get("content-length");
  if (declaredLength && (!/^\d{1,10}$/u.test(declaredLength) || Number(declaredLength) > 65_536)) {
    throw new Error("investigation search response exceeds the byte limit");
  }
  if (!response.body) return "";
  const reader = response.body.getReader();
  const decoder = new TextDecoder("utf-8", { fatal: true });
  let byteLength = 0;
  let body = "";
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      byteLength += value.byteLength;
      if (byteLength > 65_536) throw new Error("investigation search response exceeds the byte limit");
      body += decoder.decode(value, { stream: true });
    }
    return body + decoder.decode();
  } finally {
    reader.releaseLock();
  }
}
