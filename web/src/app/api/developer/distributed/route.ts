import { cookies } from "next/headers";
import { NextRequest } from "next/server";
import { sanitizeDistributedEvidence } from "@/lib/api/distributed";
import { authorizeOperationsRead, isOperationsReadDenial, proxyOperationsGET, strictOperationsQuery } from "@/lib/operations-read";
import { createRateLimitStore } from "@/lib/rate-limit";
import { readSession, sessionCookieName } from "@/lib/session";
const limiter = createRateLimitStore();
export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeOperationsRead(request, session, "developer:read", limiter);
  if (isOperationsReadDenial(authorization)) return authorization;
  const query = strictOperationsQuery(request, []);
  if (!(query instanceof URLSearchParams)) return query;
  return proxyOperationsGET(authorization.session, "/api/developer/distributed", query, sanitizeDistributedEvidence);
}
