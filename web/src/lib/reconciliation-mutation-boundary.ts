import { NextRequest, NextResponse } from "next/server";

import { isValidReconciliationIdempotencyKey } from "@/lib/api/reconciliation";
import type { RateLimitStore } from "@/lib/rate-limit";
import { hasValidCSRF, jsonError } from "@/lib/security";
import type { Session } from "@/lib/session";
import { isPrivateAPITimeout } from "@/lib/upstream-outcome";

export type ReconciliationMutationAuthorization = Readonly<{ idempotencyKey: string }>;

export async function authorizeReconciliationMutation(
  request: NextRequest,
  session: Session | null,
  rateLimit: RateLimitStore,
): Promise<ReconciliationMutationAuthorization | NextResponse> {
  if (request.method !== "POST") {
    const response = jsonError("method_not_allowed", 405);
    response.headers.set("Allow", "POST");
    return response;
  }
  if (!session) return jsonError("unauthorized", 401);
  if (!session.scopes?.includes("reconciliation:write")) return jsonError("forbidden", 403);
  if (!hasValidCSRF(request, session)) return jsonError("csrf_failed", 403);
  const idempotencyKey = request.headers.get("idempotency-key");
  if (!isValidReconciliationIdempotencyKey(idempotencyKey)) return jsonError("idempotency_key_required", 400);
  const mediaType = request.headers.get("content-type")?.split(";", 1)[0].trim().toLowerCase();
  if (mediaType !== "application/json") return jsonError("unsupported_media_type", 415);
  const decision = await rateLimit.consume(`reconciliation:${session.tenantId}:${session.subjectId}`, 6, 60);
  if (!decision.allowed) {
    const response = jsonError("rate_limited", 429);
    response.headers.set("Retry-After", String(decision.retryAfterSeconds));
    return response;
  }
  return { idempotencyKey };
}

export function isReconciliationMutationDenial(value: ReconciliationMutationAuthorization | NextResponse): value is NextResponse {
  return value instanceof NextResponse;
}

export function reconciliationPrivateHeaders(privateHeaders: Readonly<Record<string, string>>, idempotencyKey: string): Record<string, string> {
  const authorization = privateHeaders.Authorization;
  const assertion = privateHeaders["X-LedgerSync-Actor-Assertion"];
  const requestID = privateHeaders["X-Request-ID"];
  if (!authorization || !assertion || !requestID || !isValidReconciliationIdempotencyKey(idempotencyKey)) {
    throw new Error("private reconciliation mutation headers are incomplete");
  }
  return {
    Authorization: authorization,
    "X-LedgerSync-Actor-Assertion": assertion,
    "X-Request-ID": requestID,
    "Content-Type": "application/json",
    "Idempotency-Key": idempotencyKey,
  };
}

function safeHeader(value: string | null, pattern: RegExp): string | null {
  return value !== null && pattern.test(value) ? value : null;
}

export function reconciliationResponseHeaders(upstream: Headers): Record<string, string> {
  const result: Record<string, string> = { "Cache-Control": "no-store" };
  if (upstream.get("idempotent-replay") === "true") result["Idempotent-Replay"] = "true";
  const requestID = safeHeader(upstream.get("x-request-id"), /^[A-Za-z0-9._:-]{1,128}$/);
  if (requestID) result["X-Request-ID"] = requestID;
  const retryAfter = safeHeader(upstream.get("retry-after"), /^[0-9]{1,6}$/);
  if (retryAfter) result["Retry-After"] = retryAfter;
  return result;
}

export function reconciliationDispatchError(error: unknown): Readonly<{ code: string; status: number }> {
  return isPrivateAPITimeout(error)
    ? { code: "reconciliation_outcome_unknown", status: 504 }
    : { code: "temporary_unavailable", status: 503 };
}

