import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { createSession, sessionCookie, sessionCookieName, readSession } from "@/lib/session";
import { hasValidCSRF, jsonError, readBoundedJSON } from "@/lib/security";
import { toPrivateTransferRequest, type CreateTransferInput } from "@/lib/api/transfers";

export async function POST(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  if (!hasValidCSRF(request, session)) return jsonError("csrf_failed", 403);
  const idempotencyKey = request.headers.get("idempotency-key")?.trim();
  if (!idempotencyKey) return jsonError("idempotency_key_required", 400);
  const privateAPIURL = process.env.LEDGERSYNC_PRIVATE_API_URL;
  const privateAPIToken = process.env.LEDGERSYNC_PRIVATE_API_TOKEN;
  if (!privateAPIURL || !privateAPIToken) {
    // The BFF is safe-by-default: it cannot silently fall back to a browser
    // request or manufacture an outcome when its private API is unavailable.
    return jsonError("temporary_unavailable", 503);
  }

  let body: ReturnType<typeof toPrivateTransferRequest>;
  try {
    body = toPrivateTransferRequest(await readBoundedJSON<CreateTransferInput>(request));
  } catch {
    return jsonError("validation_failed", 400);
  }

  let upstream: Response;
  try {
    upstream = await fetch(`${privateAPIURL.replace(/\/$/, "")}/api/transfers`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${privateAPIToken}`,
        "Content-Type": "application/json",
        "Idempotency-Key": idempotencyKey,
        "X-Request-ID": request.headers.get("x-request-id") ?? crypto.randomUUID(),
      },
      body: JSON.stringify(body),
      cache: "no-store",
    });
  } catch {
    return jsonError("temporary_unavailable", 503);
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
