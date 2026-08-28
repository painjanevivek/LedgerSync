import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { sanitizeRecoveryIndex } from "@/lib/api/recovery";
import { authorizeOperationsRead, isOperationsReadDenial, proxyOperationsGET, strictOperationsQuery } from "@/lib/operations-read";
import { recoveryReadRateLimit } from "@/lib/recovery-read";
import { readSession, sessionCookieName } from "@/lib/session";

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeOperationsRead(request, session, "recovery:read", recoveryReadRateLimit);
  if (isOperationsReadDenial(authorization)) return authorization;
  const query = strictOperationsQuery(request, []);
  if (!(query instanceof URLSearchParams)) return query;
  return proxyOperationsGET(authorization.session, "/api/recovery/manifests", query, sanitizeRecoveryIndex);
}
