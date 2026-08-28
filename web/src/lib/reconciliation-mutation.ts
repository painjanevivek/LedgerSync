import { NextResponse } from "next/server";

import { sanitizeReconciliationUpstreamBody } from "@/lib/api/reconciliation";
import { privateAPIContext } from "@/lib/private-api";
import {
  reconciliationDispatchError,
  reconciliationPrivateHeaders,
  reconciliationResponseHeaders,
} from "@/lib/reconciliation-mutation-boundary";
import { jsonError } from "@/lib/security";
import type { Session } from "@/lib/session";
import { privateWriteTimeoutMilliseconds } from "@/lib/upstream-outcome";

export async function proxyReconciliationMutation(session: Session, idempotencyKey: string): Promise<NextResponse> {
  let connection: Awaited<ReturnType<typeof privateAPIContext>>;
  try {
    connection = await privateAPIContext(session);
  } catch {
    return jsonError("temporary_unavailable", 503);
  }

  let upstream: Response;
  try {
    upstream = await fetch(`${connection.apiURL}/api/reconciliation/runs`, {
      method: "POST",
      headers: reconciliationPrivateHeaders(connection.headers, idempotencyKey),
      body: "{}",
      cache: "no-store",
      signal: AbortSignal.timeout(privateWriteTimeoutMilliseconds),
    });
  } catch (error) {
    const failure = reconciliationDispatchError(error);
    return jsonError(failure.code, failure.status);
  }

  let raw: string;
  try {
    raw = await upstream.text();
  } catch {
    const sanitized = sanitizeReconciliationUpstreamBody(upstream.status, "");
    return NextResponse.json(sanitized.body, { status: sanitized.status, headers: reconciliationResponseHeaders(upstream.headers) });
  }
  const sanitized = sanitizeReconciliationUpstreamBody(upstream.status, raw);
  const headers = reconciliationResponseHeaders(upstream.headers);
  if (sanitized.status === 504) delete headers["Idempotent-Replay"];
  return NextResponse.json(sanitized.body, { status: sanitized.status, headers });
}
