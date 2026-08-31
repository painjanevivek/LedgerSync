import { NextRequest, NextResponse } from "next/server";

import { isWebhookReplayIdempotencyKey, isWebhookReplayIdentifier } from "@/lib/api/webhook-replay";
import type { RateLimitStore } from "@/lib/rate-limit";
import { hasValidCSRF, jsonError } from "@/lib/security";
import type { Session } from "@/lib/session";
import { isPrivateAPITimeout } from "@/lib/upstream-outcome";

export type WebhookReplayStage = "approval" | "execution";
export type WebhookReplayAuthorization = Readonly<{ idempotencyKey: string }>;

export async function authorizeWebhookReplay(
  request: NextRequest,
  session: Session | null,
  endpointId: string,
  attemptId: string,
  stage: WebhookReplayStage,
  rateLimit: RateLimitStore,
): Promise<WebhookReplayAuthorization | NextResponse> {
  if (request.method !== "POST") {
    const response = jsonError("method_not_allowed", 405);
    response.headers.set("Allow", "POST");
    return response;
  }
  if (!session) return jsonError("unauthorized", 401);
  if (!session.scopes?.includes("webhooks:replay")) return jsonError("forbidden", 403);
  if (!isWebhookReplayIdentifier(endpointId) || !isWebhookReplayIdentifier(attemptId)) return jsonError("invalid_request", 400);
  if (!hasValidCSRF(request, session)) return jsonError("csrf_failed", 403);
  const authenticatedAt = session.authenticatedAt;
  if (!authenticatedAt || authenticatedAt > Date.now() + 60_000 || Date.now() - authenticatedAt > 10 * 60_000) {
    return jsonError("step_up_required", 403);
  }
  const idempotencyKey = request.headers.get("idempotency-key");
  if (!isWebhookReplayIdempotencyKey(idempotencyKey)) return jsonError("idempotency_key_required", 400);
  const mediaType = request.headers.get("content-type")?.split(";", 1)[0].trim().toLowerCase();
  if (mediaType !== "application/json") return jsonError("unsupported_media_type", 415);
  const decision = await rateLimit.consume(`webhook-replay:${stage}:${session.tenantId}:${session.subjectId}`, 6, 60);
  if (!decision.allowed) {
    const response = jsonError("rate_limited", 429);
    response.headers.set("Retry-After", String(decision.retryAfterSeconds));
    return response;
  }
  return { idempotencyKey };
}

export function isWebhookReplayDenial(value: WebhookReplayAuthorization | NextResponse): value is NextResponse {
  return value instanceof NextResponse;
}

export function webhookReplayPrivateHeaders(privateHeaders: Readonly<Record<string, string>>, idempotencyKey: string): Record<string, string> {
  const authorization = privateHeaders.Authorization;
  const assertion = privateHeaders["X-LedgerSync-Actor-Assertion"];
  const requestID = privateHeaders["X-Request-ID"];
  if (!authorization || !assertion || !requestID || !isWebhookReplayIdempotencyKey(idempotencyKey)) {
    throw new Error("private webhook replay headers are incomplete");
  }
  return {
    Authorization: authorization,
    "X-LedgerSync-Actor-Assertion": assertion,
    "X-Request-ID": requestID,
    "Content-Type": "application/json",
    "Idempotency-Key": idempotencyKey,
  };
}

export function webhookReplayResponseHeaders(upstream: Headers): Record<string, string> {
  const headers: Record<string, string> = { "Cache-Control": "no-store" };
  if (upstream.get("idempotent-replay") === "true") headers["Idempotent-Replay"] = "true";
  const requestID = upstream.get("x-request-id");
  if (requestID && /^[A-Za-z0-9._:-]{1,128}$/.test(requestID)) headers["X-Request-ID"] = requestID;
  const retryAfter = upstream.get("retry-after");
  if (retryAfter && /^[0-9]{1,6}$/.test(retryAfter)) headers["Retry-After"] = retryAfter;
  return headers;
}

export function webhookReplayDispatchError(stage: WebhookReplayStage, error: unknown): Readonly<{ code: string; status: number }> {
  return isPrivateAPITimeout(error)
    ? { code: `${stage}_outcome_unknown`, status: 504 }
    : { code: "temporary_unavailable", status: 503 };
}
