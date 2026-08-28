import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { proxyPrivateGET } from "@/lib/private-api";
import { reconciliationMutationMaximumBytes, parseReconciliationRunRequest } from "@/lib/api/reconciliation";
import { InMemoryRateLimitStore } from "@/lib/rate-limit";
import { proxyReconciliationMutation } from "@/lib/reconciliation-mutation";
import { authorizeReconciliationMutation, isReconciliationMutationDenial } from "@/lib/reconciliation-mutation-boundary";
import { readSession, sessionCookieName } from "@/lib/session";
import { jsonError, readBoundedJSON } from "@/lib/security";

const reconciliationRateLimit = new InMemoryRateLimitStore();

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  return proxyPrivateGET(request, session, "/api/reconciliation/runs", ["cursor", "limit"]);
}

export async function POST(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeReconciliationMutation(request, session, reconciliationRateLimit);
  if (isReconciliationMutationDenial(authorization)) return authorization;
  if (!session) return jsonError("unauthorized", 401);
  try {
    parseReconciliationRunRequest(await readBoundedJSON<unknown>(request, reconciliationMutationMaximumBytes));
  } catch {
    return jsonError("validation_failed", 400);
  }
  return proxyReconciliationMutation(session, authorization.idempotencyKey);
}
