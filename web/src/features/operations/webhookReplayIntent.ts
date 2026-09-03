import { isWebhookReplayIdempotencyKey, isWebhookReplayIdentifier, webhookReplayReasonCodes, type WebhookReplayReasonCode } from "@/lib/api/webhook-replay";

export type WebhookReplayIntent = Readonly<{
  version: 1;
  tenantId: string;
  endpointId: string;
  attemptId: string;
  reasonCode: WebhookReplayReasonCode;
  approvalKey: string;
  executionKey: string;
  approvalId?: string;
  state: "review" | "approval_unknown" | "approved" | "execution_unknown" | "scheduled";
  deliveryJobId?: string;
}>;

export function webhookReplayStorageKey(tenantId: string, endpointId: string, attemptId: string): string {
  return `ledgersync:webhook-replay:v1:${encodeURIComponent(tenantId)}:${encodeURIComponent(endpointId)}:${encodeURIComponent(attemptId)}`;
}

export function newWebhookReplayIntent(tenantId: string, endpointId: string, attemptId: string, reasonCode: WebhookReplayReasonCode = "endpoint_restored"): WebhookReplayIntent {
  return {
    version: 1,
    tenantId,
    endpointId,
    attemptId,
    reasonCode,
    approvalKey: `webhook-approval-${crypto.randomUUID()}`,
    executionKey: `webhook-execution-${crypto.randomUUID()}`,
    state: "review",
  };
}

export function parseWebhookReplayIntent(raw: string | null, tenantId: string, endpointId: string, attemptId: string): WebhookReplayIntent | null {
  if (!raw || raw.length > 4_096) return null;
  try {
    const value = JSON.parse(raw) as Partial<WebhookReplayIntent>;
    if (value.version !== 1
      || value.tenantId !== tenantId
      || value.endpointId !== endpointId
      || value.attemptId !== attemptId
      || !webhookReplayReasonCodes.includes(value.reasonCode as WebhookReplayReasonCode)
      || !isWebhookReplayIdempotencyKey(value.approvalKey)
      || !isWebhookReplayIdempotencyKey(value.executionKey)
      || !["review", "approval_unknown", "approved", "execution_unknown", "scheduled"].includes(String(value.state))
      || value.approvalId !== undefined && !isWebhookReplayIdentifier(value.approvalId)
      || value.deliveryJobId !== undefined && !isWebhookReplayIdentifier(value.deliveryJobId)
      || ["approved", "execution_unknown", "scheduled"].includes(String(value.state)) && !value.approvalId
      || value.state === "scheduled" && !value.deliveryJobId) return null;
    return value as WebhookReplayIntent;
  } catch { return null; }
}
