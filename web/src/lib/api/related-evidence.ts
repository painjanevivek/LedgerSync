export const relationshipSourceTypes = ["account", "transfer", "funding", "event", "reconciliation_run", "reconciliation_mismatch", "correction"] as const;
export type RelationshipSourceType = typeof relationshipSourceTypes[number];

export const relationshipTargetTypes = [...relationshipSourceTypes, "journal", "posting", "delivery_attempt", "approval"] as const;
export type RelationshipTargetType = typeof relationshipTargetTypes[number];

export type RelatedEvidence = Readonly<{
  relationship_type: string;
  target_type: RelationshipTargetType;
  target_id: string;
  safe_label: string;
  status: string;
  occurred_at: string;
  source: "postgresql";
  freshness: "relationship_snapshot";
}>;

export type RelatedEvidencePage = Readonly<{
  source_type: RelationshipSourceType;
  source_id: string;
  relationships: readonly RelatedEvidence[];
  generated_at: string;
  truncated: boolean;
}>;

export type SanitizedRelatedEvidenceResponse = Readonly<{ status: number; body: Readonly<Record<string, unknown>> }>;

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/u;
const safeText = /^[A-Za-z0-9][A-Za-z0-9 ._:/()'-]{0,127}$/u;
const relationshipType = /^[a-z][a-z0-9_]{1,63}$/u;
const sourceTypes = new Set<string>(relationshipSourceTypes);
const targetTypes = new Set<string>(relationshipTargetTypes);
const statuses = new Set(["active", "frozen", "closed", "pending", "posted", "rejected", "requested", "approved", "compensated", "published", "dead", "retrying", "delivered", "matched", "mismatch", "failed", "cancelled", "expired", "recorded"]);

function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function exactKeys(value: Record<string, unknown>, allowed: readonly string[]) { return Object.keys(value).every((key) => allowed.includes(key)); }
function timestamp(value: unknown): value is string { return typeof value === "string" && value.length <= 64 && !Number.isNaN(Date.parse(value)); }

function sanitizeRelationship(value: unknown): RelatedEvidence | null {
  if (!isRecord(value) || !exactKeys(value, ["relationship_type", "target_type", "target_id", "safe_label", "status", "occurred_at", "source", "freshness"])) return null;
  if (typeof value.relationship_type !== "string" || !relationshipType.test(value.relationship_type)
    || typeof value.target_type !== "string" || !targetTypes.has(value.target_type)
    || typeof value.target_id !== "string" || !uuid.test(value.target_id)
    || typeof value.safe_label !== "string" || !safeText.test(value.safe_label)
    || typeof value.status !== "string" || !statuses.has(value.status)
    || !timestamp(value.occurred_at) || value.source !== "postgresql" || value.freshness !== "relationship_snapshot") return null;
  return value as RelatedEvidence;
}

function sanitizeError(status: number, value: unknown): SanitizedRelatedEvidenceResponse {
  const error = isRecord(value) && isRecord(value.error) ? value.error : undefined;
  const code = typeof error?.code === "string" ? error.code : "";
  if ([400, 401, 403, 404, 429, 503, 504].includes(status) && ["invalid_request", "validation_failed", "unauthorized", "forbidden", "not_found", "rate_limited", "evidence_unavailable", "temporary_unavailable", "upstream_timeout"].includes(code)) return { status, body: { error: { code } } };
  return { status: 503, body: { error: { code: "evidence_unavailable" } } };
}

export function sanitizeRelatedEvidence(status: number, value: unknown): SanitizedRelatedEvidenceResponse {
  if (status < 200 || status >= 300) return sanitizeError(status, value);
  if (!isRecord(value) || !exactKeys(value, ["source_type", "source_id", "relationships", "generated_at", "truncated"])
    || typeof value.source_type !== "string" || !sourceTypes.has(value.source_type)
    || typeof value.source_id !== "string" || !uuid.test(value.source_id)
    || !Array.isArray(value.relationships) || value.relationships.length > 20
    || !timestamp(value.generated_at) || typeof value.truncated !== "boolean") return sanitizeError(503, null);
  const relationships = value.relationships.map(sanitizeRelationship);
  if (relationships.some((item) => item === null)) return sanitizeError(503, null);
  return { status, body: { source_type: value.source_type, source_id: value.source_id, relationships, generated_at: value.generated_at, truncated: value.truncated } };
}

export function parseRelatedEvidenceBody(status: number, body: string): SanitizedRelatedEvidenceResponse {
  if (new TextEncoder().encode(body).byteLength > 65_536) return sanitizeError(503, null);
  try { return sanitizeRelatedEvidence(status, JSON.parse(body) as unknown); }
  catch { return sanitizeError(503, null); }
}

export async function readBoundedRelatedEvidenceBody(response: Response): Promise<string> {
  const declaredLength = response.headers.get("content-length");
  if (declaredLength && (!/^\d{1,10}$/u.test(declaredLength) || Number(declaredLength) > 65_536)) throw new Error("related evidence response exceeds the byte limit");
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
      if (byteLength > 65_536) throw new Error("related evidence response exceeds the byte limit");
      body += decoder.decode(value, { stream: true });
    }
    return body + decoder.decode();
  } finally { reader.releaseLock(); }
}
