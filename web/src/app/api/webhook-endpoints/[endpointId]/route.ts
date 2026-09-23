import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { sanitizeWebhookEndpointDetail } from "@/lib/api/operations";
import { authorizeOperationsRead, isOperationsReadDenial, proxyOperationsGET, strictOperationsQuery } from "@/lib/operations-read";
import { createRateLimitStore } from "@/lib/rate-limit";
import { jsonError } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";

const webhookEndpointDetailRateLimit = createRateLimitStore();
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export async function GET(request: NextRequest, context: { params: Promise<{ endpointId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeOperationsRead(request, session, "webhooks:read", webhookEndpointDetailRateLimit);
  if (isOperationsReadDenial(authorization)) return authorization;
  const query = strictOperationsQuery(request, []);
  if (!(query instanceof URLSearchParams)) return query;
  const { endpointId } = await context.params;
  if (!uuid.test(endpointId)) return jsonError("validation_failed", 400);
  return proxyOperationsGET(authorization.session, `/api/webhook-endpoints/${encodeURIComponent(endpointId)}`, query, sanitizeWebhookEndpointDetail);
}
