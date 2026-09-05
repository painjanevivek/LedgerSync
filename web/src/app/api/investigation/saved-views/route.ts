import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { parseSavedViewCreateInput, readBoundedSavedViewBody, sanitizeSavedView, sanitizeSavedViewPage } from "@/lib/api/saved-investigation-views";
import { authorizeInvestigationSavedViews, isInvestigationSearchDenial } from "@/lib/investigation-search-boundary";
import { privateAPIContext } from "@/lib/private-api";
import { createRateLimitStore } from "@/lib/rate-limit";
import { jsonError, readBoundedJSON } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";
import { isPrivateAPITimeout, privateReadTimeoutMilliseconds, privateWriteTimeoutMilliseconds } from "@/lib/upstream-outcome";

const savedViewRateLimit = createRateLimitStore();
const maximumRequestBytes = 8 << 10;

function responseHeaders(upstream: Response) {
  const headers = new Headers({ "Cache-Control": "no-store" });
  const requestID = upstream.headers.get("x-request-id");
  if (requestID && /^[A-Za-z0-9._:-]{1,128}$/u.test(requestID)) headers.set("X-Request-ID", requestID);
  const retryAfter = upstream.headers.get("retry-after");
  if (retryAfter && /^[0-9]{1,6}$/u.test(retryAfter)) headers.set("Retry-After", retryAfter);
  const location = upstream.headers.get("location");
  if (location && /^\/api\/investigation\/saved-views\/[0-9a-f-]{36}$/u.test(location)) headers.set("Location", location);
  return headers;
}

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeInvestigationSavedViews(request, session, savedViewRateLimit, false);
  if (isInvestigationSearchDenial(authorization)) return authorization;
  if (request.nextUrl.searchParams.size > 0) return jsonError("invalid_request", 400);
  try {
    const connection = await privateAPIContext(authorization.session, request.headers.get("x-request-id") ?? undefined);
    const upstream = await fetch(`${connection.apiURL}/api/investigation/saved-views`, { headers: connection.headers, cache: "no-store", signal: AbortSignal.timeout(privateReadTimeoutMilliseconds) });
    const sanitized = sanitizeSavedViewPage(upstream.status, await readBoundedSavedViewBody(upstream));
    return NextResponse.json(sanitized.body, { status: sanitized.status, headers: responseHeaders(upstream) });
  } catch (error) {
    return jsonError(isPrivateAPITimeout(error) ? "upstream_timeout" : "temporary_unavailable", isPrivateAPITimeout(error) ? 504 : 503);
  }
}

export async function POST(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeInvestigationSavedViews(request, session, savedViewRateLimit, true);
  if (isInvestigationSearchDenial(authorization)) return authorization;
  if (request.nextUrl.searchParams.size > 0) return jsonError("invalid_request", 400);
  let input: ReturnType<typeof parseSavedViewCreateInput>;
  try {
    input = parseSavedViewCreateInput(await readBoundedJSON<unknown>(request, maximumRequestBytes));
  } catch {
    return jsonError("invalid_request", 400);
  }
  try {
    const connection = await privateAPIContext(authorization.session, request.headers.get("x-request-id") ?? undefined);
    const upstream = await fetch(`${connection.apiURL}/api/investigation/saved-views`, { method: "POST", headers: { ...connection.headers, "Content-Type": "application/json" }, body: JSON.stringify(input), cache: "no-store", signal: AbortSignal.timeout(privateWriteTimeoutMilliseconds) });
    const sanitized = sanitizeSavedView(upstream.status, await readBoundedSavedViewBody(upstream));
    return NextResponse.json(sanitized.body, { status: sanitized.status, headers: responseHeaders(upstream) });
  } catch (error) {
    return jsonError(isPrivateAPITimeout(error) ? "upstream_timeout" : "temporary_unavailable", isPrivateAPITimeout(error) ? 504 : 503);
  }
}
