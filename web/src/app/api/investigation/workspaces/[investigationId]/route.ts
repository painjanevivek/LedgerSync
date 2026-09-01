import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { readBoundedWorkspaceBody, sanitizeWorkspace } from "@/lib/api/investigation-workspaces";
import { authorizeInvestigationWorkspaces, isInvestigationSearchDenial } from "@/lib/investigation-search-boundary";
import { privateAPIContext } from "@/lib/private-api";
import { InMemoryRateLimitStore } from "@/lib/rate-limit";
import { jsonError } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";
import { isPrivateAPITimeout, privateReadTimeoutMilliseconds } from "@/lib/upstream-outcome";

const rateLimit = new InMemoryRateLimitStore();
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/u;

export async function GET(request: NextRequest, { params }: Readonly<{ params: Promise<{ investigationId: string }> }>) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeInvestigationWorkspaces(request, session, rateLimit, false);
  if (isInvestigationSearchDenial(authorization)) return authorization;
  const investigationId = (await params).investigationId.toLowerCase();
  if (!uuid.test(investigationId) || request.nextUrl.searchParams.size > 0) return jsonError("invalid_request", 400);
  try {
    const connection = await privateAPIContext(authorization.session, request.headers.get("x-request-id") ?? undefined);
    const upstream = await fetch(`${connection.apiURL}/api/investigation/workspaces/${investigationId}`, { headers: connection.headers, cache: "no-store", signal: AbortSignal.timeout(privateReadTimeoutMilliseconds) });
    const sanitized = sanitizeWorkspace(upstream.status, await readBoundedWorkspaceBody(upstream));
    const headers = new Headers({ "Cache-Control": "no-store" });
    const requestID = upstream.headers.get("x-request-id"); if (requestID && /^[A-Za-z0-9._:-]{1,128}$/u.test(requestID)) headers.set("X-Request-ID", requestID);
    return NextResponse.json(sanitized.body, { status: sanitized.status, headers });
  } catch (error) {
    return jsonError(isPrivateAPITimeout(error) ? "upstream_timeout" : "temporary_unavailable", isPrivateAPITimeout(error) ? 504 : 503);
  }
}
