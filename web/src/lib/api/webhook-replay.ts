const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const idempotencyPattern = /^[\x21-\x7e]{16,255}$/;

export const webhookReplayMaximumBytes = 4_096;
export const webhookReplayReasonCodes = [
  "endpoint_restored",
  "receiver_capacity_restored",
  "transient_outage_resolved",
] as const;

export type WebhookReplayReasonCode = (typeof webhookReplayReasonCodes)[number];
export type WebhookReplayApproval = Readonly<{ approval_id: string; status: "approved" }>;
export type WebhookReplayResult = Readonly<{ delivery_job_id: string; status: "scheduled" }>;

export function isWebhookReplayIdentifier(value: unknown): value is string {
  return typeof value === "string" && uuidPattern.test(value);
}

export function isWebhookReplayIdempotencyKey(value: unknown): value is string {
  return typeof value === "string" && idempotencyPattern.test(value);
}

function exactObject(value: unknown, keys: readonly string[]): value is Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return false;
  const actual = Object.keys(value).sort();
  const expected = [...keys].sort();
  return actual.length === expected.length && actual.every((key, index) => key === expected[index]);
}

export function parseWebhookReplayApprovalRequest(value: unknown): Readonly<{ reason_code: WebhookReplayReasonCode }> {
  if (!exactObject(value, ["reason_code"]) || !webhookReplayReasonCodes.includes(value.reason_code as WebhookReplayReasonCode)) {
    throw new Error("invalid webhook replay approval request");
  }
  return { reason_code: value.reason_code as WebhookReplayReasonCode };
}

export function parseWebhookReplayRequest(value: unknown): Readonly<{ approval_id: string }> {
  if (!exactObject(value, ["approval_id"]) || !isWebhookReplayIdentifier(value.approval_id)) {
    throw new Error("invalid webhook replay request");
  }
  return { approval_id: value.approval_id };
}

export function isWebhookReplayApproval(value: unknown): value is WebhookReplayApproval {
  return exactObject(value, ["approval_id", "status"])
    && isWebhookReplayIdentifier(value.approval_id)
    && value.status === "approved";
}

export function isWebhookReplayResult(value: unknown): value is WebhookReplayResult {
  return exactObject(value, ["delivery_job_id", "status"])
    && isWebhookReplayIdentifier(value.delivery_job_id)
    && value.status === "scheduled";
}

export function sanitizeWebhookReplayUpstreamBody(
  stage: "approval" | "execution",
  status: number,
  raw: string,
): Readonly<{ status: number; body: WebhookReplayApproval | WebhookReplayResult | { error: { code: string } } }> {
  if (new TextEncoder().encode(raw).byteLength > webhookReplayMaximumBytes) {
    return { status: 504, body: { error: { code: `${stage}_outcome_unknown` } } };
  }
  let value: unknown;
  try { value = JSON.parse(raw); } catch { value = {}; }
  if (stage === "approval" && status >= 200 && status < 300 && isWebhookReplayApproval(value)) return { status, body: value };
  if (stage === "execution" && status >= 200 && status < 300 && isWebhookReplayResult(value)) return { status, body: value };
  if (status >= 200 && status < 300) return { status: 504, body: { error: { code: `${stage}_outcome_unknown` } } };
  const errorCode = typeof value === "object" && value !== null
    && typeof (value as { error?: { code?: unknown } }).error?.code === "string"
    && /^[a-z0-9_]{3,64}$/.test((value as { error: { code: string } }).error.code)
      ? (value as { error: { code: string } }).error.code
      : "temporary_unavailable";
  return { status: status >= 400 && status <= 599 ? status : 503, body: { error: { code: errorCode } } };
}
