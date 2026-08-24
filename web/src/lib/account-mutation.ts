import { NextResponse } from "next/server";

import {
  accountMutationDispatchError,
  accountMutationPrivateHeaders,
  accountMutationResponseHeaders,
  type AccountMutationMethod,
} from "@/lib/account-mutation-boundary";
import {
  sanitizeAccountUpstreamBody,
  sanitizeUnusableAccountUpstream,
  type SanitizedAccountUpstream,
} from "@/lib/api/accounts";
import { privateAPIContext } from "@/lib/private-api";
import { jsonError } from "@/lib/security";
import type { Session } from "@/lib/session";
import { privateWriteTimeoutMilliseconds } from "@/lib/upstream-outcome";

function accountMutationResponse(upstream: Response, sanitized: SanitizedAccountUpstream): NextResponse {
  const headers = accountMutationResponseHeaders(upstream.headers);
  if (sanitized.status === 504) delete headers["Idempotent-Replay"];
  return NextResponse.json(sanitized.body, { status: sanitized.status, headers });
}

export async function proxyAccountMutation(
  session: Session,
  method: AccountMutationMethod,
  path: string,
  body: Readonly<Record<string, unknown>>,
  idempotencyKey: string,
): Promise<NextResponse> {
  let connection: Awaited<ReturnType<typeof privateAPIContext>>;
  try {
    // Request IDs and all private authorization headers are generated at the
    // server boundary; no caller-controlled private header is forwarded.
    connection = await privateAPIContext(session);
  } catch {
    return jsonError("temporary_unavailable", 503);
  }

  let upstream: Response;
  try {
    upstream = await fetch(`${connection.apiURL}${path}`, {
      method,
      headers: accountMutationPrivateHeaders(connection.headers, idempotencyKey),
      body: JSON.stringify(body),
      cache: "no-store",
      signal: AbortSignal.timeout(privateWriteTimeoutMilliseconds),
    });
  } catch (error) {
    // A dispatch timeout cannot distinguish a rejected request from a durable
    // commit whose response was lost. The browser must retry the same body and
    // key; reporting a confirmed failure could cause a duplicate intent.
    const failure = accountMutationDispatchError(error);
    return jsonError(failure.code, failure.status);
  }

  let raw: string;
  try {
    raw = await upstream.text();
  } catch {
    return accountMutationResponse(upstream, sanitizeUnusableAccountUpstream(upstream.status));
  }
  return accountMutationResponse(upstream, sanitizeAccountUpstreamBody(upstream.status, raw));
}
