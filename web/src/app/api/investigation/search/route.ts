import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { parseInvestigationSearchBody, readBoundedInvestigationSearchBody } from "@/lib/api/investigation-search";
import { authorizeInvestigationSearch, isInvestigationSearchDenial, parseInvestigationSearchQuery } from "@/lib/investigation-search-boundary";
import { privateAPIContext } from "@/lib/private-api";
import { InMemoryRateLimitStore } from "@/lib/rate-limit";
import { jsonError } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";
import { isPrivateAPITimeout, privateReadTimeoutMilliseconds } from "@/lib/upstream-outcome";

const searchRateLimit = new InMemoryRateLimitStore();

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeInvestigationSearch(request, session, searchRateLimit);
  if (isInvestigationSearchDenial(authorization)) return authorization;
  const parsed = parseInvestigationSearchQuery(request.nextUrl.searchParams);
  if (!parsed) return jsonError("validation_failed", 400);
  try {
    const connection = await privateAPIContext(authorization.session, request.headers.get("x-request-id") ?? undefined);
    const upstream = await fetch(`${connection.apiURL}/api/investigation/search?${parsed.query}`, {
      headers: connection.headers,
      cache: "no-store",
      signal: AbortSignal.timeout(privateReadTimeoutMilliseconds),
    });
    const sanitized = parseInvestigationSearchBody(upstream.status, await readBoundedInvestigationSearchBody(upstream));
    const response = NextResponse.json(sanitized.body, { status: sanitized.status, headers: { "Cache-Control": "no-store" } });
    const requestID = upstream.headers.get("x-request-id");
    if (requestID && /^[A-Za-z0-9._:-]{1,128}$/u.test(requestID)) response.headers.set("X-Request-ID", requestID);
    const retryAfter = upstream.headers.get("retry-after");
    if (retryAfter && /^[0-9]{1,6}$/u.test(retryAfter)) response.headers.set("Retry-After", retryAfter);
    return response;
  } catch (error) {
    return jsonError(isPrivateAPITimeout(error) ? "upstream_timeout" : "temporary_unavailable", isPrivateAPITimeout(error) ? 504 : 503);
  }
}
