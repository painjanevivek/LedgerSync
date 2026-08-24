import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { sanitizeLocalOrientation } from "@/lib/api/orientation";
import { authorizeOperationsRead, isOperationsReadDenial, proxyOperationsGET, strictOperationsQuery } from "@/lib/operations-read";
import { InMemoryRateLimitStore } from "@/lib/rate-limit";
import { readSession, sessionCookieName } from "@/lib/session";

const orientationRateLimit = new InMemoryRateLimitStore();

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeOperationsRead(request, session, "local:read", orientationRateLimit);
  if (isOperationsReadDenial(authorization)) return authorization;
  const query = strictOperationsQuery(request, []);
  if (!(query instanceof URLSearchParams)) return query;
  return proxyOperationsGET(authorization.session, "/api/local/orientation", query, sanitizeLocalOrientation);
}
