import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { parseOrientationPreferenceInput, sanitizeLocalOrientation } from "@/lib/api/orientation";
import { privateAPIContext } from "@/lib/private-api";
import { hasValidCSRF, jsonError, readBoundedJSON } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";
import { isPrivateAPITimeout, privateWriteTimeoutMilliseconds } from "@/lib/upstream-outcome";

const preferenceMaximumBytes = 4 << 10;

export async function PUT(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  if (!session.scopes?.includes("local:write")) return jsonError("forbidden", 403);
  if (!hasValidCSRF(request, session)) return jsonError("csrf_failed", 403);
  if (request.nextUrl.searchParams.size > 0) return jsonError("invalid_request", 400);

  let body: ReturnType<typeof parseOrientationPreferenceInput>;
  try {
    body = parseOrientationPreferenceInput(await readBoundedJSON<unknown>(request, preferenceMaximumBytes));
  } catch {
    return jsonError("invalid_request", 400);
  }

  try {
    const connection = await privateAPIContext(session, request.headers.get("x-request-id") ?? undefined);
    const upstream = await fetch(`${connection.apiURL}/api/local/orientation/preferences`, {
      method: "PUT",
      headers: { ...connection.headers, "Content-Type": "application/json" },
      body: JSON.stringify(body),
      cache: "no-store",
      signal: AbortSignal.timeout(privateWriteTimeoutMilliseconds),
    });
    const sanitized = sanitizeLocalOrientation(upstream.status, await upstream.json().catch(() => ({})));
    const response = NextResponse.json(sanitized.body, { status: sanitized.status, headers: { "Cache-Control": "no-store" } });
    const requestID = upstream.headers.get("x-request-id");
    if (requestID) response.headers.set("X-Request-ID", requestID);
    const retryAfter = upstream.headers.get("retry-after");
    if (retryAfter) response.headers.set("Retry-After", retryAfter);
    return response;
  } catch (error) {
    return jsonError(isPrivateAPITimeout(error) ? "upstream_timeout" : "temporary_unavailable", isPrivateAPITimeout(error) ? 504 : 503);
  }
}
