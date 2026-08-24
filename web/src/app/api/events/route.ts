import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { sanitizeEventPage } from "@/lib/api/operations";
import { authorizeOperationsRead, isOperationsReadDenial, proxyOperationsGET, strictOperationsQuery } from "@/lib/operations-read";
import { InMemoryRateLimitStore } from "@/lib/rate-limit";
import { readSession, sessionCookieName } from "@/lib/session";

const eventsRateLimit = new InMemoryRateLimitStore();
const eventFilters = ["eventType", "state", "relatedId", "correlationId", "from", "to", "cursor", "limit"] as const;

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeOperationsRead(request, session, "events:read", eventsRateLimit);
  if (isOperationsReadDenial(authorization)) return authorization;
  const query = strictOperationsQuery(request, eventFilters);
  if (!(query instanceof URLSearchParams)) return query;
  return proxyOperationsGET(authorization.session, "/api/events", query, sanitizeEventPage);
}
