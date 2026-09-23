import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { sanitizeTransferExplainability } from "@/lib/api/orientation";
import { authorizeOperationsRead, isOperationsReadDenial, proxyOperationsGET, strictOperationsQuery } from "@/lib/operations-read";
import { createRateLimitStore } from "@/lib/rate-limit";
import { jsonError } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";
import { canonicalUUID } from "@/lib/canonical-uuid";

const explainabilityRateLimit = createRateLimitStore();
const requiredScopes = ["transfers:read", "events:read", "reconciliation:read"] as const;

export async function GET(request: NextRequest, context: { params: Promise<{ transferId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeOperationsRead(request, session, "explainability:read", explainabilityRateLimit);
  if (isOperationsReadDenial(authorization)) return authorization;
  if (!requiredScopes.every((scope) => authorization.session.scopes?.includes(scope))) return jsonError("forbidden", 403);
  const query = strictOperationsQuery(request, []);
  if (!(query instanceof URLSearchParams)) return query;
  const transferId = canonicalUUID((await context.params).transferId);
  if (!transferId) return jsonError("validation_failed", 400);
  return proxyOperationsGET(authorization.session, `/api/transfers/${encodeURIComponent(transferId)}/explainability`, query, sanitizeTransferExplainability);
}
