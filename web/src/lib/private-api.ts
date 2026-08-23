import "server-only";

import { NextRequest, NextResponse } from "next/server";

import { createActorAssertion } from "@/lib/actor-assertion";
import type { Session } from "@/lib/session";
import { jsonError } from "@/lib/security";
import { getPrivateAPIWorkloadCredential } from "@/lib/workload-credential";

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
  const query = new URLSearchParams();
  for (const key of allowedQuery) {
    const value = request.nextUrl.searchParams.get(key)?.trim();
    if (value) {
      if (value.length > 256) return jsonError("validation_failed", 400);
      query.set(key, value);
    }
  }
  try {
    const connection = await privateAPIContext(session, request.headers.get("x-request-id") ?? undefined);
    const suffix = query.size ? `?${query}` : "";
    const upstream = await fetch(`${connection.apiURL}${path}${suffix}`, {
      headers: connection.headers,
      cache: "no-store",
      signal: AbortSignal.timeout(8_000),
    });
    const response = new NextResponse(await upstream.text(), { status: upstream.status, headers: { "Content-Type": upstream.headers.get("content-type") ?? "application/json", "Cache-Control": "no-store" } });
    const requestId = upstream.headers.get("x-request-id"); if (requestId) response.headers.set("X-Request-ID", requestId);
    return response;
  } catch { return jsonError("temporary_unavailable", 503); }
}
