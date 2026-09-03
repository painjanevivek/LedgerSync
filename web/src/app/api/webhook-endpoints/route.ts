import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { sanitizeWebhookEndpointPage } from "@/lib/api/operations";
import { authorizeOperationsRead, isOperationsReadDenial, proxyOperationsGET, strictOperationsQuery } from "@/lib/operations-read";
import { InMemoryRateLimitStore } from "@/lib/rate-limit";
import { readSession, sessionCookieName } from "@/lib/session";
import { parseWebhookBFFSearchParams } from "@/lib/page-query/operations";
import { jsonError } from "@/lib/security";

const webhookEndpointRateLimit = new InMemoryRateLimitStore();
const webhookEndpointFilters = ["status", "eventType", "cursor", "limit"] as const;

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeOperationsRead(request, session, "webhooks:read", webhookEndpointRateLimit);
  if (isOperationsReadDenial(authorization)) return authorization;
  if (!parseWebhookBFFSearchParams(request.nextUrl.searchParams).ok) return jsonError("validation_failed", 400);
  const query = strictOperationsQuery(request, webhookEndpointFilters);
  if (!(query instanceof URLSearchParams)) return query;
  return proxyOperationsGET(authorization.session, "/api/webhook-endpoints", query, sanitizeWebhookEndpointPage);
}
