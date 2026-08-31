"use client";

import { useRef, useState } from "react";

import { isWebhookReplayApproval, isWebhookReplayResult, type WebhookReplayReasonCode } from "@/lib/api/webhook-replay";

export type WebhookReplayOutcome =
  | Readonly<{ kind: "approval"; approvalId: string; replayed: boolean; requestReference: string }>
  | Readonly<{ kind: "scheduled"; deliveryJobId: string; replayed: boolean; requestReference: string }>
  | Readonly<{ kind: "unknown" | "denied" | "error" | "unavailable"; code: string; message: string; requestReference: string }>;

function errorCode(value: unknown): string {
  if (typeof value !== "object" || value === null) return "temporary_unavailable";
  const code = (value as { error?: { code?: unknown } }).error?.code;
  return typeof code === "string" ? code : "temporary_unavailable";
}

export function useWebhookReplay(csrfToken: string) {
  const [pending, setPending] = useState<"approval" | "execution" | null>(null);
  const inFlight = useRef(false);

  async function send(endpointId: string, attemptId: string, stage: "approval" | "execution", idempotencyKey: string, input: Readonly<{ reasonCode: WebhookReplayReasonCode } | { approvalId: string }>): Promise<WebhookReplayOutcome> {
    const localReference = crypto.randomUUID();
    if (inFlight.current) return { kind: "unavailable", code: "request_in_flight", message: "This replay command is already in flight. Wait for its authoritative response.", requestReference: localReference };
    inFlight.current = true; setPending(stage);
    const suffix = stage === "approval" ? "replay-approvals" : "replay";
    const body = "reasonCode" in input ? { reason_code: input.reasonCode } : { approval_id: input.approvalId };
    try {
      const response = await fetch(`/api/webhook-endpoints/${encodeURIComponent(endpointId)}/deliveries/${encodeURIComponent(attemptId)}/${suffix}`, {
        method: "POST",
        cache: "no-store",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "Idempotency-Key": idempotencyKey, "X-Request-ID": localReference },
        body: JSON.stringify(body),
        signal: AbortSignal.timeout(8_000),
      });
      const value: unknown = await response.json().catch(() => ({}));
      const requestReference = response.headers.get("X-Request-ID") ?? localReference;
      const replayed = response.headers.get("Idempotent-Replay") === "true";
      if (stage === "approval" && response.ok && isWebhookReplayApproval(value)) return { kind: "approval", approvalId: value.approval_id, replayed, requestReference };
      if (stage === "execution" && response.ok && isWebhookReplayResult(value)) return { kind: "scheduled", deliveryJobId: value.delivery_job_id, replayed, requestReference };
      const code = errorCode(value);
      if (response.status === 504 || code.endsWith("_outcome_unknown")) return { kind: "unknown", code, message: `LedgerSync cannot prove whether this ${stage} command was recorded. Retry only the retained exact command and key.`, requestReference };
      if (response.status === 401 || response.status === 403) return { kind: "denied", code, message: code === "step_up_required" ? "Recent authentication is required before this sensitive recovery command." : "This operator is not authorized for webhook replay recovery.", requestReference };
      if (response.status === 400 || response.status === 409 || response.status === 415) return { kind: "error", code, message: code === "replay_separation_required" ? "The operator who executes replay must be different from the operator who approved it." : "The replay command was rejected without scheduling a new delivery.", requestReference };
      return { kind: "unavailable", code, message: response.status === 429 ? "Replay recovery is temporarily rate limited. Retain the exact command and retry after the stated interval." : "Replay recovery is temporarily unavailable; no new delivery is being inferred.", requestReference };
    } catch {
      return { kind: "unknown", code: `${stage}_connection_lost`, message: `The connection ended after submission. Retry only this retained ${stage} command and key.`, requestReference: localReference };
    } finally { inFlight.current = false; setPending(null); }
  }
  return { pending, send };
}
