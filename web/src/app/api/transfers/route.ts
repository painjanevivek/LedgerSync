import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { createSession, sessionCookie, sessionCookieName, readSession } from "@/lib/session";
import { hasValidCSRF, jsonError, readBoundedJSON } from "@/lib/security";
import { toPrivateTransferRequest, type CreateTransferInput } from "@/lib/api/transfers";
import { privateAPIContext, proxyPrivateGET } from "@/lib/private-api";
import { isPrivateAPITimeout, privateWriteTimeoutMilliseconds } from "@/lib/upstream-outcome";
import { parseTransferSearchParams, transferBFFQueryRules } from "@/lib/page-query/transfers";

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

  let upstream: Response;
  try {
    const connection = await privateAPIContext(session, request.headers.get("x-request-id") ?? undefined);
    upstream = await fetch(`${connection.apiURL}/api/transfers`, {
      method: "POST",
      headers: {
        ...connection.headers,
        "Content-Type": "application/json",
        "Idempotency-Key": idempotencyKey,
        "X-LedgerSync-Request-Reference": requestReference,
      },
      body: JSON.stringify(body),
      cache: "no-store",
      signal: AbortSignal.timeout(privateWriteTimeoutMilliseconds),
    });
  } catch (error) {
    // Once a transfer request leaves the BFF, a timeout is an unknown outcome,
    // never a confirmed failure. Clients must retry the identical payload with
    // the same idempotency key to learn the committed result safely.
    const response = jsonError(isPrivateAPITimeout(error) ? "transfer_outcome_unknown" : "temporary_unavailable", isPrivateAPITimeout(error) ? 504 : 503);
    response.headers.set("X-LedgerSync-Request-Reference", requestReference);
    return response;
  }

  const payload = await upstream.text();
  const response = new NextResponse(payload, {
    status: upstream.status,
    headers: {
      "Content-Type": upstream.headers.get("content-type") ?? "application/json",
      "Cache-Control": "no-store",
    },
  });
  const replay = upstream.headers.get("idempotent-replay");
  if (replay) response.headers.set("Idempotent-Replay", replay);
  const requestID = upstream.headers.get("x-request-id");
  if (requestID) response.headers.set("X-Request-ID", requestID);
  response.headers.set("X-LedgerSync-Request-Reference", upstream.headers.get("x-ledgersync-request-reference") ?? requestReference);
  const retryAfter = upstream.headers.get("retry-after");
  if (retryAfter) response.headers.set("Retry-After", retryAfter);
  const serializedRequirements = upstream.headers.get("x-ledgersync-consistency-requirements");
  if (serializedRequirements && upstream.ok) {
    try {
      const requirements = JSON.parse(serializedRequirements) as Record<string, string>;
      if (Object.entries(requirements).length <= 10 && Object.entries(requirements).every(([accountId, token]) => accountId.length > 0 && typeof token === "string" && token.length <= 2048)) {
        response.cookies.set(sessionCookie(createSession({ ...session, consistencyRequirements: { ...session.consistencyRequirements, ...requirements } })));
      }
    } catch {
      // A malformed private requirement must not be exposed to the browser or
      // treated as a successful consistency guarantee.
      return jsonError("temporary_unavailable", 503);
    }
  }
  return response;
}
