import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { parseRelatedEvidenceBody, readBoundedRelatedEvidenceBody, relationshipSourceTypes } from "@/lib/api/related-evidence";
import { authorizeInvestigationRelationships, isInvestigationSearchDenial } from "@/lib/investigation-search-boundary";
import { privateAPIContext } from "@/lib/private-api";
import { InMemoryRateLimitStore } from "@/lib/rate-limit";
import { jsonError } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";
import { isPrivateAPITimeout, privateReadTimeoutMilliseconds } from "@/lib/upstream-outcome";

const relationshipRateLimit = new InMemoryRateLimitStore();
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/u;
const sourceTypes = new Set<string>(relationshipSourceTypes);

export async function GET(request: NextRequest, context: { params: Promise<{ recordType: string; recordId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeInvestigationRelationships(request, session, relationshipRateLimit);
  if (isInvestigationSearchDenial(authorization)) return authorization;
  if ([...request.nextUrl.searchParams.keys()].length > 0) return jsonError("validation_failed", 400);
  const { recordType, recordId: rawRecordId } = await context.params;
  const recordId = rawRecordId.toLowerCase();
  if (!sourceTypes.has(recordType) || !uuid.test(recordId)) return jsonError("validation_failed", 400);
  try {
    const connection = await privateAPIContext(authorization.session, request.headers.get("x-request-id") ?? undefined);
    const upstream = await fetch(`${connection.apiURL}/api/investigation/related/${encodeURIComponent(recordType)}/${encodeURIComponent(recordId)}`, { headers: connection.headers, cache: "no-store", signal: AbortSignal.timeout(privateReadTimeoutMilliseconds) });
    const sanitized = parseRelatedEvidenceBody(upstream.status, await readBoundedRelatedEvidenceBody(upstream));
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
