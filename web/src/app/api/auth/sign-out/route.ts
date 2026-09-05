import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { expiredSessionCookie, readSession, revokeOpaqueSession, sessionCookieName } from "@/lib/session";
import { hasValidCSRF, jsonError } from "@/lib/security";

export async function POST(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  if (!hasValidCSRF(request, session)) return jsonError("csrf_failed", 403);
  const handle = request.headers.get("x-ledgersync-session-handle");
  try {
    if (!handle || !(await revokeOpaqueSession(handle))) return jsonError("temporary_unavailable", 503);
  } catch {
    return jsonError("temporary_unavailable", 503);
  }
  const response = new NextResponse(null, { status: 204, headers: { "Cache-Control": "no-store" } });
  response.cookies.set(expiredSessionCookie());
  return response;
}
