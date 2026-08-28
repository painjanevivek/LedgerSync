import "server-only";

import { NextRequest, NextResponse } from "next/server";

import { createActorAssertion } from "@/lib/actor-assertion";
import type { Session } from "@/lib/session";
import { jsonError } from "@/lib/security";
import { getPrivateAPIWorkloadCredential } from "@/lib/workload-credential";
import { isPrivateAPITimeout, privateReadTimeoutMilliseconds } from "@/lib/upstream-outcome";

export async function privateAPIContext(session: Session, requestID?: string) {
  const apiURL = process.env.LEDGERSYNC_PRIVATE_API_URL?.trim();
  if (!apiURL) throw new Error("private API URL is unavailable");
  return {
    apiURL: apiURL.replace(/\/$/, ""),
    headers: {
      Authorization: `Bearer ${await getPrivateAPIWorkloadCredential()}`,
      "X-LedgerSync-Actor-Assertion": createActorAssertion(session),
      "X-Request-ID": requestID ?? crypto.randomUUID(),
    },
  };
}

export async function proxyPrivateGET(request: NextRequest, session: Session, path: string, allowedQuery: readonly string[] = []) {
  const permitted = new Set(allowedQuery);
  const query = new URLSearchParams();
  for (const [key] of request.nextUrl.searchParams) {
    if (!permitted.has(key) || request.nextUrl.searchParams.getAll(key).length !== 1) {
      return jsonError("invalid_request", 400);
    }
  }
  for (const key of allowedQuery) {
    const raw = request.nextUrl.searchParams.get(key);
    if (raw === null) continue;
    const value = raw.trim();
    const maximumLength = key === "cursor" ? 2_048 : 256;
    if (!value || value.length > maximumLength) return jsonError("invalid_request", 400);
    query.set(key, value);
  }
  try {
    const connection = await privateAPIContext(session, request.headers.get("x-request-id") ?? undefined);
    const suffix = query.size ? `?${query}` : "";
    const upstream = await fetch(`${connection.apiURL}${path}${suffix}`, {
      headers: connection.headers,
      cache: "no-store",
      signal: AbortSignal.timeout(privateReadTimeoutMilliseconds),
    });
    const response = new NextResponse(await upstream.text(), { status: upstream.status, headers: { "Content-Type": upstream.headers.get("content-type") ?? "application/json", "Cache-Control": "no-store" } });
    const requestId = upstream.headers.get("x-request-id"); if (requestId) response.headers.set("X-Request-ID", requestId);
    const retryAfter = upstream.headers.get("retry-after"); if (retryAfter) response.headers.set("Retry-After", retryAfter);
    return response;
  } catch (error) { return jsonError(isPrivateAPITimeout(error) ? "upstream_timeout" : "temporary_unavailable", isPrivateAPITimeout(error) ? 504 : 503); }
}
