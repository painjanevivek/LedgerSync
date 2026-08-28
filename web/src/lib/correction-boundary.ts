import { NextRequest, NextResponse } from "next/server";

import { isCorrectionIdempotencyKey } from "@/lib/api/corrections";
import { hasValidCSRF, jsonError } from "@/lib/security";
import type { Session } from "@/lib/session";

export function authorizeCorrectionMutation(
  request: NextRequest,
  session: Session | null,
  scope: "corrections:write" | "corrections:approve",
  idempotencyRequired: boolean,
  jsonBodyRequired: boolean,
): Readonly<{ idempotencyKey?: string }> | NextResponse {
  if (request.method !== "POST") {
    const response = jsonError("method_not_allowed", 405);
    response.headers.set("Allow", "POST");
    return response;
  }
  if (!session) return jsonError("unauthorized", 401);
  if (!session.scopes?.includes(scope)) return jsonError("forbidden", 403);
  if (!hasValidCSRF(request, session)) return jsonError("csrf_failed", 403);
  if (
    jsonBodyRequired &&
    request.headers
      .get("content-type")
      ?.split(";", 1)[0]
      .trim()
      .toLowerCase() !== "application/json"
  )
    return jsonError("unsupported_media_type", 415);
  const idempotencyKey = request.headers.get("idempotency-key");
  if (idempotencyRequired && !isCorrectionIdempotencyKey(idempotencyKey))
    return jsonError("idempotency_key_required", 400);
  return {
    idempotencyKey: isCorrectionIdempotencyKey(idempotencyKey)
      ? idempotencyKey
      : undefined,
  };
}

export function isCorrectionDenial(
  value: Readonly<{ idempotencyKey?: string }> | NextResponse,
): value is NextResponse {
  return value instanceof NextResponse;
}
