import { NextResponse } from "next/server";

import { sanitizeWebhookReplayUpstreamBody } from "@/lib/api/webhook-replay";
import { privateAPIContext } from "@/lib/private-api";
import type { Session } from "@/lib/session";
import { privateWriteTimeoutMilliseconds } from "@/lib/upstream-outcome";
import { webhookReplayDispatchError, webhookReplayPrivateHeaders, webhookReplayResponseHeaders, type WebhookReplayStage } from "@/lib/webhook-replay-boundary";

export async function proxyWebhookReplayMutation(
  session: Session,
  endpointId: string,
  attemptId: string,
  stage: WebhookReplayStage,
  idempotencyKey: string,
  body: Readonly<Record<string, string>>,
  requestID?: string,
): Promise<NextResponse> {
  let connection: Awaited<ReturnType<typeof privateAPIContext>>;
  try { connection = await privateAPIContext(session, requestID); }
  catch { return NextResponse.json({ error: { code: "temporary_unavailable" } }, { status: 503 }); }
  const suffix = stage === "approval" ? "replay-approvals" : "replay";
  let upstream: Response;
  try {
    upstream = await fetch(`${connection.apiURL}/api/developer/webhooks/${encodeURIComponent(endpointId)}/deliveries/${encodeURIComponent(attemptId)}/${suffix}`, {
      method: "POST",
      headers: webhookReplayPrivateHeaders(connection.headers, idempotencyKey),
      body: JSON.stringify(body),
      cache: "no-store",
      signal: AbortSignal.timeout(privateWriteTimeoutMilliseconds),
    });
  } catch (error) {
    const failure = webhookReplayDispatchError(stage, error);
    return NextResponse.json({ error: { code: failure.code } }, { status: failure.status, headers: { "Cache-Control": "no-store" } });
  }
  let raw = "";
  try { raw = await upstream.text(); } catch { /* sanitizer produces an unknown outcome */ }
  const sanitized = sanitizeWebhookReplayUpstreamBody(stage, upstream.status, raw);
  const headers = webhookReplayResponseHeaders(upstream.headers);
  if (sanitized.status === 504) delete headers["Idempotent-Replay"];
  return NextResponse.json(sanitized.body, { status: sanitized.status, headers });
}
