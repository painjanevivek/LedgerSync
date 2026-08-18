import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import { readSession, sessionCookieName } from "@/lib/session";
import { jsonError } from "@/lib/security";

/**
 * Exposes the minimum browser-safe session context required by the same-origin
 * operator console. The authentication cookie itself remains HttpOnly.
 */
export async function GET() {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);

  return NextResponse.json({
    subject_id: session.subjectId,
    tenant_id: session.tenantId,
    csrf_token: session.csrfToken,
    scopes: session.scopes ?? [],
  }, { headers: { "Cache-Control": "no-store" } });
}
