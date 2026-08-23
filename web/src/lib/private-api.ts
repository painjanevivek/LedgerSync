import "server-only";

import { NextRequest, NextResponse } from "next/server";

import { createActorAssertion } from "@/lib/actor-assertion";
import type { Session } from "@/lib/session";
import { jsonError } from "@/lib/security";

export async function proxyPrivateGET(request: NextRequest, session: Session, path: string, allowedQuery: readonly string[] = []) {
  const apiURL = process.env.LEDGERSYNC_PRIVATE_API_URL;
  const token = process.env.LEDGERSYNC_PRIVATE_API_TOKEN;
  if (!apiURL || !token) return jsonError("temporary_unavailable", 503);
  const query = new URLSearchParams();
  for (const key of allowedQuery) {
    const value = request.nextUrl.searchParams.get(key)?.trim();
    if (value) {
      if (value.length > 256) return jsonError("validation_failed", 400);
      query.set(key, value);
    }
  }
  let assertion: string;
  try { assertion = createActorAssertion(session); } catch { return jsonError("temporary_unavailable", 503); }
  try {
    const suffix = query.size ? `?${query}` : "";
    const upstream = await fetch(`${apiURL.replace(/\/$/, "")}${path}${suffix}`, {
      headers: { Authorization: `Bearer ${token}`, "X-LedgerSync-Actor-Assertion": assertion, "X-Request-ID": request.headers.get("x-request-id") ?? crypto.randomUUID() },
      cache: "no-store",
      signal: AbortSignal.timeout(8_000),
    });
    const response = new NextResponse(await upstream.text(), { status: upstream.status, headers: { "Content-Type": upstream.headers.get("content-type") ?? "application/json", "Cache-Control": "no-store" } });
    const requestId = upstream.headers.get("x-request-id"); if (requestId) response.headers.set("X-Request-ID", requestId);
    return response;
  } catch { return jsonError("temporary_unavailable", 503); }
}
