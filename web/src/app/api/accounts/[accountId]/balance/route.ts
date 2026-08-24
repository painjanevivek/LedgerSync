import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { readSession, sessionCookieName } from "@/lib/session";
import { jsonError } from "@/lib/security";
import { privateAPIContext } from "@/lib/private-api";

export async function GET(request: NextRequest, context: { params: Promise<{ accountId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  const { accountId } = await context.params;
  const normalizedAccountId = accountId.trim();
  if (!normalizedAccountId) return jsonError("validation_failed", 400);
  const requirement = session.consistencyRequirements?.[normalizedAccountId];
  let upstream: Response;
  try {
    const connection = await privateAPIContext(session, request.headers.get("x-request-id") ?? undefined);
    upstream = await fetch(`${connection.apiURL}/api/accounts/${encodeURIComponent(normalizedAccountId)}/balance`, { method: "GET", headers: { ...connection.headers, ...(requirement ? { "X-LedgerSync-Consistency-Requirement": requirement } : {}) }, cache: "no-store", signal: AbortSignal.timeout(8_000) });
  } catch { return jsonError("temporary_unavailable", 503); }
  const response = new NextResponse(await upstream.text(), { status: upstream.status, headers: { "Content-Type": upstream.headers.get("content-type") ?? "application/json", "Cache-Control": "no-store" } });
  const requestID = upstream.headers.get("x-request-id");
  if (requestID) response.headers.set("X-Request-ID", requestID);
  return response;
}
