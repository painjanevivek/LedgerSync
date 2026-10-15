import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { sanitizeEventDetail } from "@/lib/api/operations";
import { authorizeOperationsRead, isOperationsReadDenial, proxyOperationsGET, strictOperationsQuery } from "@/lib/operations-read";
import { createRateLimitStore } from "@/lib/rate-limit";
import { jsonError } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";
import { traceViewerURL } from "@/lib/trace-viewer";

const eventDetailRateLimit = createRateLimitStore();
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export async function GET(request: NextRequest, context: { params: Promise<{ eventId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeOperationsRead(request, session, "events:read", eventDetailRateLimit);
  if (isOperationsReadDenial(authorization)) return authorization;
  const query = strictOperationsQuery(request, []);
  if (!(query instanceof URLSearchParams)) return query;
  const { eventId } = await context.params;
  if (!uuid.test(eventId)) return jsonError("validation_failed", 400);
  return proxyOperationsGET(authorization.session, `/api/events/${encodeURIComponent(eventId)}`, query, (status, value) => {
    const sanitized = sanitizeEventDetail(status, value);
    if (sanitized.status === 200) {
      const traceId = sanitized.body.diagnostic_trace_id;
      return { ...sanitized, body: { ...sanitized.body, trace_viewer_available: typeof traceId === "string" && traceViewerURL(traceId, process.env.LEDGERSYNC_TRACE_VIEWER_BASE_URL) !== null } };
    }
    return sanitized;
  });
}
