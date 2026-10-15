import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { sessionCookieName, readSession } from "@/lib/session";
import { hasValidCSRF, jsonError, readBoundedJSON } from "@/lib/security";
import { toPrivateTransferRequest, type CreateTransferInput } from "@/lib/api/transfers";
import { proxyPrivateGET } from "@/lib/private-api";
import { parseTransferSearchParams, transferBFFQueryRules } from "@/lib/page-query/transfers";
import { proxyTransferMutation } from "@/lib/transfer-mutation";

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  if (!parseTransferSearchParams(request.nextUrl.searchParams, transferBFFQueryRules).ok)
    return jsonError("invalid_request", 400);
  return proxyPrivateGET(request, session, "/api/transfers", ["cursor", "limit", "accountId", "status", "q", "from", "to"]);
}

export async function POST(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  if (!hasValidCSRF(request, session)) return jsonError("csrf_failed", 403);
  const idempotencyKey = request.headers.get("idempotency-key")?.trim();
  if (!idempotencyKey) return jsonError("idempotency_key_required", 400);
  const requestReference = request.headers.get("x-ledgersync-request-reference")?.trim().toLowerCase();
  if (!requestReference || !/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/u.test(requestReference)) return jsonError("validation_failed", 400);
  let body: ReturnType<typeof toPrivateTransferRequest>;
  try {
    body = toPrivateTransferRequest(await readBoundedJSON<CreateTransferInput>(request));
  } catch {
    return jsonError("validation_failed", 400);
  }

  return proxyTransferMutation(session, "/api/transfers", body, idempotencyKey, requestReference, request.headers.get("x-request-id") ?? undefined, request.headers.get("x-ledgersync-session-handle") ?? undefined);
}
