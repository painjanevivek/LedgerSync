import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import { isLocalSession, readLocalAccessConfiguration } from "@/lib/local-access";
import { readSession, sessionCookieName } from "@/lib/session";
import { jsonError } from "@/lib/security";

/**
 * Exposes the minimum browser-safe session context required by the same-origin
 * operator console. The authentication cookie itself remains HttpOnly.
 */
export async function GET() {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  let local = false;
  try {
    const configuration = readLocalAccessConfiguration();
    local = isLocalSession(session, configuration);
  } catch {
    return jsonError("local_access_configuration_invalid", 503);
  }
  if (!session) return jsonError("unauthorized", 401);

  return NextResponse.json({
    subject_id: session.subjectId,
    tenant_id: session.tenantId,
    csrf_token: session.csrfToken,
    scopes: session.scopes ?? [],
    environment: local ? "local" : "production",
    operator_label: local ? "Local operator" : "Authorized operator",
    tenant_label: local ? "My Ledger Workspace" : "Ledger tenant",
  }, { headers: { "Cache-Control": "no-store" } });
}
