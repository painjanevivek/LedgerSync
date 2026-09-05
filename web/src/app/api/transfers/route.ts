import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { sessionCookieName, readSession, updateOpaqueSessionConsistency } from "@/lib/session";
import { hasValidCSRF, jsonError, readBoundedJSON } from "@/lib/security";
import { toPrivateTransferRequest, type CreateTransferInput } from "@/lib/api/transfers";
import { privateAPIContext, proxyPrivateGET } from "@/lib/private-api";
import { isPrivateAPITimeout, privateWriteTimeoutMilliseconds } from "@/lib/upstream-outcome";
import { parseTransferSearchParams, transferBFFQueryRules } from "@/lib/page-query/transfers";
import { parseConsistencyRequirements } from "@/lib/committed-response";

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
      },
      body: JSON.stringify(body),
      cache: "no-store",
      signal: AbortSignal.timeout(privateWriteTimeoutMilliseconds),
    });
  } catch (error) {
    // Once a transfer request leaves the BFF, a timeout is an unknown outcome,
    // never a confirmed failure. Clients must retry the identical payload with
    // the same idempotency key to learn the committed result safely.
    return jsonError(isPrivateAPITimeout(error) ? "transfer_outcome_unknown" : "temporary_unavailable", isPrivateAPITimeout(error) ? 504 : 503);
  }

  let payload: string;
  try {
    payload = await upstream.text();
  } catch {
    if (!upstream.ok) return jsonError("temporary_unavailable", 503);
    payload = JSON.stringify({
      outcome: "committed",
      metadata_status: "unavailable",
      recovery_method: "POST",
      reuse_idempotency_key: true,
    });
  }
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
  const retryAfter = upstream.headers.get("retry-after");
  if (retryAfter) response.headers.set("Retry-After", retryAfter);
  const upstreamMetadataStatus = upstream.headers.get("x-ledgersync-metadata-status");
  if (upstreamMetadataStatus && ["complete", "partial", "unavailable"].includes(upstreamMetadataStatus)) {
    response.headers.set("X-LedgerSync-Metadata-Status", upstreamMetadataStatus);
  }
  const serializedRequirements = upstream.headers.get("x-ledgersync-consistency-requirements");
  if (serializedRequirements && upstream.ok) {
    const requirements = parseConsistencyRequirements(serializedRequirements);
    const sessionHandle = request.headers.get("x-ledgersync-session-handle");
    let metadataStored = false;
    try {
      metadataStored = Boolean(requirements && sessionHandle && await updateOpaqueSessionConsistency(sessionHandle, requirements));
    } catch {
      metadataStored = false;
    }
    if (!metadataStored) {
      response.headers.delete("X-LedgerSync-Consistency-Requirements");
      response.headers.set("X-LedgerSync-Metadata-Status", "unavailable");
    }
  }
  return response;
}
