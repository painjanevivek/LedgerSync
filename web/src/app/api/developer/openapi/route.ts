import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { proxyDeveloperOpenAPI } from "@/lib/developer-read";
import { authorizeOperationsRead, isOperationsReadDenial, strictOperationsQuery } from "@/lib/operations-read";
import { createRateLimitStore } from "@/lib/rate-limit";
import { readSession, sessionCookieName } from "@/lib/session";

const developerOpenAPIRateLimit = createRateLimitStore();

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeOperationsRead(request, session, "developer:read", developerOpenAPIRateLimit);
  if (isOperationsReadDenial(authorization)) return authorization;
  const query = strictOperationsQuery(request, []);
  if (!(query instanceof URLSearchParams)) return query;
  return proxyDeveloperOpenAPI(authorization.session);
}
