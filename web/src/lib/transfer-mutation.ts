import "server-only";
import { NextResponse } from "next/server";
import { type Session, sessionCookie } from "@/lib/session";
import { jsonError } from "@/lib/security";
import { privateAPIContext } from "@/lib/private-api";
import { isPrivateAPITimeout, privateWriteTimeoutMilliseconds } from "@/lib/upstream-outcome";
import { applyConsistencySessionMetadata } from "@/lib/committed-response";
export async function proxyTransferMutation(session: Session, path: string, body?: unknown, idempotencyKey?: string, requestReference?: string, requestID?: string) {
  let upstream: Response;
  try {
    const connection = await privateAPIContext(session, requestID);
    upstream = await fetch(`${connection.apiURL}${path}`, {
      method: "POST",
      headers: {
        ...connection.headers,
        "Content-Type": "application/json",
        ...(idempotencyKey ? { "Idempotency-Key": idempotencyKey } : {}),
        ...(requestReference ? { "X-LedgerSync-Request-Reference": requestReference } : {}),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
      cache: "no-store",
      signal: AbortSignal.timeout(privateWriteTimeoutMilliseconds),
    });
  } catch (error) {
    // Once a transfer request leaves the BFF, a timeout is an unknown outcome,
    // never a confirmed failure. Clients must retry the identical payload with
    // the same idempotency key to learn the committed result safely.
    const response = jsonError(isPrivateAPITimeout(error) ? "transfer_outcome_unknown" : "temporary_unavailable", isPrivateAPITimeout(error) ? 504 : 503);
    if (requestReference) response.headers.set("X-LedgerSync-Request-Reference", requestReference);
    return response;
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
  const responseRequestID = upstream.headers.get("x-request-id");
  if (responseRequestID) response.headers.set("X-Request-ID", responseRequestID);
  response.headers.set("X-LedgerSync-Request-Reference", upstream.headers.get("x-ledgersync-request-reference") ?? requestReference ?? "");
  const retryAfter = upstream.headers.get("retry-after");
  if (retryAfter) response.headers.set("Retry-After", retryAfter);
  const upstreamMetadataStatus = upstream.headers.get("x-ledgersync-metadata-status");
  if (upstreamMetadataStatus && ["complete", "partial", "unavailable"].includes(upstreamMetadataStatus)) {
    response.headers.set("X-LedgerSync-Metadata-Status", upstreamMetadataStatus);
  }
  const serializedRequirements = upstream.headers.get("x-ledgersync-consistency-requirements");
  if (serializedRequirements && upstream.ok) {
    const metadataStatus = applyConsistencySessionMetadata(
      serializedRequirements,
      session,
      (signedSession) => response.cookies.set(sessionCookie(signedSession)),
    );
    if (metadataStatus === "unavailable") {
      response.headers.delete("X-LedgerSync-Consistency-Requirements");
      response.headers.set("X-LedgerSync-Metadata-Status", "unavailable");
    }
  }
  return response;
}
