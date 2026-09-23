import "server-only";

import { NextResponse } from "next/server";

import { canonicalizeUUIDPathSegments } from "@/lib/canonical-uuid";
import { privateAPIContext } from "@/lib/private-api";
import { jsonError } from "@/lib/security";
import type { Session } from "@/lib/session";
import {
  isPrivateAPITimeout,
  privateWriteTimeoutMilliseconds,
} from "@/lib/upstream-outcome";

export async function proxyCorrectionMutation(
  session: Session,
  path: string,
  body?: Readonly<Record<string, unknown>>,
  idempotencyKey?: string,
) {
  try {
    const connection = await privateAPIContext(session);
    const headers: Record<string, string> = { ...connection.headers };
    if (body) headers["Content-Type"] = "application/json";
    if (idempotencyKey) headers["Idempotency-Key"] = idempotencyKey;
    const upstream = await fetch(`${connection.apiURL}${canonicalizeUUIDPathSegments(path)}`, {
      method: "POST",
      headers,
      body: body ? JSON.stringify(body) : undefined,
      cache: "no-store",
      signal: AbortSignal.timeout(privateWriteTimeoutMilliseconds),
    });
    const response = new NextResponse(await upstream.text(), {
      status: upstream.status,
      headers: {
        "Content-Type":
          upstream.headers.get("content-type") ?? "application/json",
        "Cache-Control": "no-store",
      },
    });
    if (upstream.headers.get("idempotent-replay") === "true")
      response.headers.set("Idempotent-Replay", "true");
    const requestID = upstream.headers.get("x-request-id");
    if (requestID && /^[A-Za-z0-9._:-]{1,128}$/.test(requestID))
      response.headers.set("X-Request-ID", requestID);
    const retryAfter = upstream.headers.get("retry-after");
    if (retryAfter && /^[0-9]{1,6}$/.test(retryAfter))
      response.headers.set("Retry-After", retryAfter);
    return response;
  } catch (error) {
    return jsonError(
      isPrivateAPITimeout(error)
        ? "correction_outcome_unknown"
        : "temporary_unavailable",
      isPrivateAPITimeout(error) ? 504 : 503,
    );
  }
}
