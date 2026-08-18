import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { readSession, sessionCookieName } from "@/lib/session";
import { jsonError } from "@/lib/security";

export async function GET(request: NextRequest, context: { params: Promise<{ accountId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  const { accountId } = await context.params;
  const normalizedAccountId = accountId.trim();
  if (!normalizedAccountId) return jsonError("validation_failed", 400);
  const privateAPIURL = process.env.LEDGERSYNC_PRIVATE_API_URL;
  const privateAPIToken = process.env.LEDGERSYNC_PRIVATE_API_TOKEN;
  if (!privateAPIURL || !privateAPIToken) return jsonError("temporary_unavailable", 503);
  const requirement = session.consistencyRequirements?.[normalizedAccountId];
  let upstream: Response;
  try {
    upstream = await fetch(`${privateAPIURL.replace(/\/$/, "")}/api/accounts/${encodeURIComponent(normalizedAccountId)}/balance`, { method: "GET", headers: { Authorization: `Bearer ${privateAPIToken}`, ...(requirement ? { "X-LedgerSync-Consistency-Requirement": requirement } : {}), "X-Request-ID": request.headers.get("x-request-id") ?? crypto.randomUUID() }, cache: "no-store" });
  } catch { return jsonError("temporary_unavailable", 503); }
  const response = new NextResponse(await upstream.text(), { status: upstream.status, headers: { "Content-Type": upstream.headers.get("content-type") ?? "application/json", "Cache-Control": "no-store" } });
  const requestID = upstream.headers.get("x-request-id");
  if (requestID) response.headers.set("X-Request-ID", requestID);
  return response;
}
