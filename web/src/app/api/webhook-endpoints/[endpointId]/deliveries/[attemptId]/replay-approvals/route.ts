import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { parseWebhookReplayApprovalRequest, webhookReplayMaximumBytes } from "@/lib/api/webhook-replay";
import { createRateLimitStore } from "@/lib/rate-limit";
import { readSession, sessionCookieName } from "@/lib/session";
import { jsonError, readBoundedJSON } from "@/lib/security";
import { authorizeWebhookReplay, isWebhookReplayDenial } from "@/lib/webhook-replay-boundary";
import { proxyWebhookReplayMutation } from "@/lib/webhook-replay-mutation";

const replayApprovalRateLimit = createRateLimitStore();

export async function POST(request: NextRequest, { params }: Readonly<{ params: Promise<{ endpointId: string; attemptId: string }> }>) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const { endpointId, attemptId } = await params;
  const authorization = await authorizeWebhookReplay(request, session, endpointId, attemptId, "approval", replayApprovalRateLimit);
  if (isWebhookReplayDenial(authorization)) return authorization;
  if (!session) return jsonError("unauthorized", 401);
  let body: Readonly<{ reason_code: string }>;
  try { body = parseWebhookReplayApprovalRequest(await readBoundedJSON<unknown>(request, webhookReplayMaximumBytes)); }
  catch { return jsonError("validation_failed", 400); }
  return proxyWebhookReplayMutation(session, endpointId, attemptId, "approval", authorization.idempotencyKey, body, request.headers.get("x-request-id") ?? undefined);
}
