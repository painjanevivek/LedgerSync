import { NextRequest, NextResponse } from "next/server";

import { isValidAccountIdempotencyKey } from "@/lib/api/accounts";
import { hasValidCSRF, jsonError } from "@/lib/security";
import type { Session } from "@/lib/session";
import { isPrivateAPITimeout } from "@/lib/upstream-outcome";

export type AccountMutationMethod = "POST" | "PATCH";

export type AccountMutationAuthorization = Readonly<{
  idempotencyKey: string;
}>;

export function authorizeAccountMutation(
  request: NextRequest,
  session: Session | null,
  expectedMethod: AccountMutationMethod,
): AccountMutationAuthorization | NextResponse {
  if (request.method !== expectedMethod) {
    const response = jsonError("method_not_allowed", 405);
    response.headers.set("Allow", expectedMethod);
    return response;
  }
  if (!session) return jsonError("unauthorized", 401);
  if (!session.scopes?.includes("accounts:write")) return jsonError("forbidden", 403);
  if (!hasValidCSRF(request, session)) return jsonError("csrf_failed", 403);
  const idempotencyKey = request.headers.get("idempotency-key");
  if (!isValidAccountIdempotencyKey(idempotencyKey)) return jsonError("idempotency_key_required", 400);
  const mediaType = request.headers.get("content-type")?.split(";", 1)[0].trim().toLowerCase();
  if (mediaType !== "application/json") return jsonError("unsupported_media_type", 415);
  return { idempotencyKey };
}

export function isAccountMutationDenial(value: AccountMutationAuthorization | NextResponse): value is NextResponse {
  return value instanceof NextResponse;
}

export function accountMutationPrivateHeaders(privateHeaders: Readonly<Record<string, string>>, idempotencyKey: string): Record<string, string> {
  const authorization = privateHeaders.Authorization;
  const assertion = privateHeaders["X-LedgerSync-Actor-Assertion"];
  const requestID = privateHeaders["X-Request-ID"];
  if (!authorization || !assertion || !requestID || !isValidAccountIdempotencyKey(idempotencyKey)) {
    throw new Error("private account mutation headers are incomplete");
  }
  return {
    Authorization: authorization,
    "X-LedgerSync-Actor-Assertion": assertion,
    "X-Request-ID": requestID,
    "Content-Type": "application/json",
    "Idempotency-Key": idempotencyKey,
  };
}

function safeResponseHeader(value: string | null, pattern: RegExp): string | null {
  return value !== null && pattern.test(value) ? value : null;
}

export function accountMutationResponseHeaders(upstreamHeaders: Headers): Record<string, string> {
  const headers: Record<string, string> = { "Cache-Control": "no-store" };
  if (upstreamHeaders.get("idempotent-replay") === "true") headers["Idempotent-Replay"] = "true";
  const requestID = safeResponseHeader(upstreamHeaders.get("x-request-id"), /^[A-Za-z0-9._:-]{1,128}$/);
  if (requestID) headers["X-Request-ID"] = requestID;
  const retryAfter = safeResponseHeader(upstreamHeaders.get("retry-after"), /^[0-9]{1,6}$/);
  if (retryAfter) headers["Retry-After"] = retryAfter;
  return headers;
}

export function accountMutationDispatchError(error: unknown): Readonly<{ code: string; status: number }> {
  return isPrivateAPITimeout(error)
    ? { code: "account_command_outcome_unknown", status: 504 }
    : { code: "temporary_unavailable", status: 503 };
}
