import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import { createSession, readSession, sessionCookie, sessionCookieName } from "@/lib/session";
import { createDemoSession, readDemoConfiguration } from "@/lib/demo";
import { jsonError } from "@/lib/security";

/**
 * Exposes the minimum browser-safe session context required by the same-origin
 * operator console. The authentication cookie itself remains HttpOnly.
 */
export async function GET() {
  let session = readSession((await cookies()).get(sessionCookieName)?.value);
  let demo = false;
  try {
    const configuration = readDemoConfiguration();
    if (!session && configuration.enabled) {
      session = createDemoSession(configuration);
      demo = true;
    } else if (session && configuration.enabled && session.subjectId === configuration.subjectId && session.tenantId === configuration.tenantId) {
      demo = true;
    }
  } catch {
    return jsonError("demo_configuration_invalid", 503);
  }
  if (!session) return jsonError("unauthorized", 401);

  const response = NextResponse.json({
    subject_id: session.subjectId,
    tenant_id: session.tenantId,
    csrf_token: session.csrfToken,
    scopes: session.scopes ?? [],
    environment: demo ? "demo" : "production",
    operator_label: demo ? "Demo finance operator" : "Authorized operator",
    tenant_label: demo ? "Meridian Labs · Demo" : "Ledger tenant",
  }, { headers: { "Cache-Control": "no-store" } });
  if (demo) response.cookies.set(sessionCookie(createSession(session)));
  return response;
}
