import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { sanitizeLocalDiagnostics } from "@/lib/api/operations";
import { isLocalSession, readLocalAccessConfiguration } from "@/lib/local-access";
import { authorizeOperationsRead, isOperationsReadDenial, proxyOperationsGET, strictOperationsQuery } from "@/lib/operations-read";
import { InMemoryRateLimitStore } from "@/lib/rate-limit";
import { jsonError, readPublicOrigin } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";

const diagnosticsRateLimit = new InMemoryRateLimitStore();

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  try {
    if (!isLocalSession(session, readLocalAccessConfiguration())) {
      return jsonError("forbidden", 403);
    }
  } catch {
    return jsonError("local_access_configuration_invalid", 503);
  }
  const authorization = await authorizeOperationsRead(request, session, "local:read", diagnosticsRateLimit);
  if (isOperationsReadDenial(authorization)) return authorization;
  const query = strictOperationsQuery(request, []);
  if (!(query instanceof URLSearchParams)) return query;
  let publicOrigin: string;
  try { publicOrigin = readPublicOrigin().origin; }
  catch { return Response.json({ error: { code: "temporary_unavailable" } }, { status: 503, headers: { "Cache-Control": "no-store" } }); }
  return proxyOperationsGET(authorization.session, "/api/local/diagnostics", query, (status, value) => sanitizeLocalDiagnostics(status, value, publicOrigin));
}
