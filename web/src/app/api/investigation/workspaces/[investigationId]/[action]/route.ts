import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { parseWorkspaceHandoffInput, parseWorkspaceStatusInput, readBoundedWorkspaceBody, sanitizeWorkspaceReceipt } from "@/lib/api/investigation-workspaces";
import { authorizeInvestigationWorkspaces, isInvestigationSearchDenial } from "@/lib/investigation-search-boundary";
import { privateAPIContext } from "@/lib/private-api";
import { InMemoryRateLimitStore } from "@/lib/rate-limit";
import { jsonError, readBoundedJSON } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";
import { isPrivateAPITimeout, privateWriteTimeoutMilliseconds } from "@/lib/upstream-outcome";

const rateLimit = new InMemoryRateLimitStore();
const maximumRequestBytes = 4 << 10;
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/u;
const actions = new Set(["handoff", "close", "reopen"]);

export async function POST(request: NextRequest, { params }: Readonly<{ params: Promise<{ investigationId: string; action: string }> }>) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeInvestigationWorkspaces(request, session, rateLimit, true);
  if (isInvestigationSearchDenial(authorization)) return authorization;
  const route = await params; const investigationId = route.investigationId.toLowerCase();
  if (!uuid.test(investigationId) || !actions.has(route.action) || request.nextUrl.searchParams.size > 0) return jsonError("invalid_request", 400);
  let input: ReturnType<typeof parseWorkspaceStatusInput> | ReturnType<typeof parseWorkspaceHandoffInput>;
  try { const body = await readBoundedJSON<unknown>(request, maximumRequestBytes); input = route.action === "handoff" ? parseWorkspaceHandoffInput(body) : parseWorkspaceStatusInput(body); }
  catch { return jsonError("invalid_request", 400); }
  try {
    const connection = await privateAPIContext(authorization.session, request.headers.get("x-request-id") ?? undefined);
    const upstream = await fetch(`${connection.apiURL}/api/investigation/workspaces/${investigationId}/${route.action}`, { method: "POST", headers: { ...connection.headers, "Content-Type": "application/json" }, body: JSON.stringify(input), cache: "no-store", signal: AbortSignal.timeout(privateWriteTimeoutMilliseconds) });
    const sanitized = sanitizeWorkspaceReceipt(upstream.status, await readBoundedWorkspaceBody(upstream));
    const headers = new Headers({ "Cache-Control": "no-store" });
    const requestID = upstream.headers.get("x-request-id"); if (requestID && /^[A-Za-z0-9._:-]{1,128}$/u.test(requestID)) headers.set("X-Request-ID", requestID);
    const retryAfter = upstream.headers.get("retry-after"); if (retryAfter && /^[0-9]{1,6}$/u.test(retryAfter)) headers.set("Retry-After", retryAfter);
    return NextResponse.json(sanitized.body, { status: sanitized.status, headers });
  } catch (error) {
    return jsonError(isPrivateAPITimeout(error) ? "upstream_timeout" : "temporary_unavailable", isPrivateAPITimeout(error) ? 504 : 503);
  }
}
