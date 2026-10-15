import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { parseSavedViewRenameInput, readBoundedSavedViewBody, sanitizeSavedView } from "@/lib/api/saved-investigation-views";
import { canonicalUUID } from "@/lib/canonical-uuid";
import { authorizeInvestigationSavedViews, isInvestigationSearchDenial } from "@/lib/investigation-search-boundary";
import { privateAPIContext } from "@/lib/private-api";
import { InMemoryRateLimitStore } from "@/lib/rate-limit";
import { jsonError, readBoundedJSON } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";
import { isPrivateAPITimeout, privateWriteTimeoutMilliseconds } from "@/lib/upstream-outcome";

const savedViewMutationRateLimit = new InMemoryRateLimitStore();
const maximumRequestBytes = 4 << 10;
const versionTag = /^"[1-9][0-9]{0,18}"$/u;

async function authorize(request: NextRequest, savedViewId: string) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeInvestigationSavedViews(request, session, savedViewMutationRateLimit, true);
  if (isInvestigationSearchDenial(authorization)) return authorization;
  if (!savedViewId || request.nextUrl.searchParams.size > 0) return jsonError("invalid_request", 400);
  return authorization;
}

function headers(upstream: Response) {
  const result = new Headers({ "Cache-Control": "no-store" });
  const requestID = upstream.headers.get("x-request-id");
  if (requestID && /^[A-Za-z0-9._:-]{1,128}$/u.test(requestID)) result.set("X-Request-ID", requestID);
  const retryAfter = upstream.headers.get("retry-after");
  if (retryAfter && /^[0-9]{1,6}$/u.test(retryAfter)) result.set("Retry-After", retryAfter);
  return result;
}

export async function PUT(request: NextRequest, { params }: Readonly<{ params: Promise<{ savedViewId: string }> }>) {
  const savedViewId = canonicalUUID((await params).savedViewId) ?? "";
  const authorization = await authorize(request, savedViewId);
  if (authorization instanceof NextResponse) return authorization;
  let input: ReturnType<typeof parseSavedViewRenameInput>;
  try { input = parseSavedViewRenameInput(await readBoundedJSON<unknown>(request, maximumRequestBytes)); }
  catch { return jsonError("invalid_request", 400); }
  try {
    const connection = await privateAPIContext(authorization.session, request.headers.get("x-request-id") ?? undefined);
    const upstream = await fetch(`${connection.apiURL}/api/investigation/saved-views/${savedViewId}`, { method: "PUT", headers: { ...connection.headers, "Content-Type": "application/json" }, body: JSON.stringify(input), cache: "no-store", signal: AbortSignal.timeout(privateWriteTimeoutMilliseconds) });
    const sanitized = sanitizeSavedView(upstream.status, await readBoundedSavedViewBody(upstream));
    return NextResponse.json(sanitized.body, { status: sanitized.status, headers: headers(upstream) });
  } catch (error) {
    return jsonError(isPrivateAPITimeout(error) ? "upstream_timeout" : "temporary_unavailable", isPrivateAPITimeout(error) ? 504 : 503);
  }
}

export async function DELETE(request: NextRequest, { params }: Readonly<{ params: Promise<{ savedViewId: string }> }>) {
  const savedViewId = canonicalUUID((await params).savedViewId) ?? "";
  const authorization = await authorize(request, savedViewId);
  if (authorization instanceof NextResponse) return authorization;
  const ifMatch = request.headers.get("if-match");
  const contentLength = request.headers.get("content-length");
  if (!ifMatch || !versionTag.test(ifMatch) || (contentLength && contentLength !== "0")) return jsonError("invalid_request", 400);
  try {
    const connection = await privateAPIContext(authorization.session, request.headers.get("x-request-id") ?? undefined);
    const upstream = await fetch(`${connection.apiURL}/api/investigation/saved-views/${savedViewId}`, { method: "DELETE", headers: { ...connection.headers, "If-Match": ifMatch }, cache: "no-store", signal: AbortSignal.timeout(privateWriteTimeoutMilliseconds) });
    if (upstream.status === 204) return new NextResponse(null, { status: 204, headers: headers(upstream) });
    const sanitized = sanitizeSavedView(upstream.status, await readBoundedSavedViewBody(upstream));
    return NextResponse.json(sanitized.body, { status: sanitized.status, headers: headers(upstream) });
  } catch (error) {
    return jsonError(isPrivateAPITimeout(error) ? "upstream_timeout" : "temporary_unavailable", isPrivateAPITimeout(error) ? 504 : 503);
  }
}
