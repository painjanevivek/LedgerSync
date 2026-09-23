import "server-only";

import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { isLiveUUID, sanitizeLiveSuccess } from "@/lib/api/live-investigation";
import { privateAPIContext } from "@/lib/private-api";
import { createRateLimitStore, rateLimitResponse } from "@/lib/rate-limit";
import { hasValidCSRF, jsonError, readBoundedJSON } from "@/lib/security";
import { readSession, sessionCookieName, type Session } from "@/lib/session";
import { isPrivateAPITimeout, privateReadTimeoutMilliseconds, privateWriteTimeoutMilliseconds } from "@/lib/upstream-outcome";

const liveRateLimit = createRateLimitStore();
const safeErrors = new Set(["invalid_request", "not_found", "forbidden", "unauthorized", "rate_limited", "temporary_unavailable", "collaboration_unavailable", "investigation_room_conflict", "finding_conflicts_with_transfer"]);

export function liveInvestigationEnabled(environment: Readonly<Record<string, string | undefined>> = process.env): boolean {
  const deployment = (environment.LEDGERSYNC_DEPLOYMENT_ENV ?? environment.NODE_ENV ?? "development").trim().toLowerCase();
  return environment.LEDGERSYNC_LIVE_INVESTIGATION_ENABLED === "true" && deployment === "development";
}

function authorized(session: Session, requireWrite: boolean): boolean {
  const roles = new Set(session.roles ?? []);
  const scopes = new Set(session.scopes ?? []);
  return (roles.has("tenant:operator") || roles.has("tenant:admin"))
    && ["investigation:collaborate", "investigation:read", "transfers:read"].every((scope) => scopes.has(scope))
    && (!requireWrite || scopes.has("investigation:write"));
}

export async function liveSession(request: NextRequest, mutation: boolean, requireWrite = mutation): Promise<Session | NextResponse> {
  if (!liveInvestigationEnabled()) return jsonError("not_found", 404);
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  if (!authorized(session, requireWrite)) return jsonError("forbidden", 403);
  if (mutation && !hasValidCSRF(request, session)) return jsonError("csrf_failed", 403);
  const decision = await liveRateLimit.consume(`${session.tenantId}:${session.subjectId}:${mutation ? "write" : "read"}`, mutation ? 120 : 300, 60);
  return rateLimitResponse(decision) ?? session;
}

export async function proxyLiveRequest(request: NextRequest, session: Session, privatePath: string, method: "GET" | "POST", body?: unknown): Promise<NextResponse> {
  try {
    const connection = await privateAPIContext(session, request.headers.get("x-request-id") ?? undefined);
    const headers: Record<string, string> = { ...connection.headers };
    if (method === "POST") {
      headers["Content-Type"] = "application/json";
      const key = request.headers.get("idempotency-key")?.trim();
      if (key) headers["Idempotency-Key"] = key;
    }
    const upstream = await fetch(`${connection.apiURL}${privatePath}`, {
      method,
      headers,
      ...(method === "POST" ? { body: JSON.stringify(body ?? {}) } : {}),
      cache: "no-store",
      signal: AbortSignal.timeout(method === "GET" ? privateReadTimeoutMilliseconds : privateWriteTimeoutMilliseconds),
    });
    const raw = await upstream.text();
    let responseBody: unknown = {};
    try { responseBody = raw ? JSON.parse(raw) as unknown : {}; } catch { responseBody = {}; }
    if (!upstream.ok) {
      const candidate = responseBody as { error?: { code?: unknown } };
      const code = typeof candidate.error?.code === "string" && safeErrors.has(candidate.error.code) ? candidate.error.code : "collaboration_unavailable";
      const response = jsonError(code, upstream.status >= 400 && upstream.status <= 599 ? upstream.status : 503);
      const retry = upstream.headers.get("retry-after"); if (retry && /^[0-9]{1,6}$/u.test(retry)) response.headers.set("Retry-After", retry);
      return response;
    }
    if (upstream.status === 204) return new NextResponse(null, { status: 204, headers: { "Cache-Control": "no-store" } });
    const sanitized = sanitizeLiveSuccess(privatePath.split("?")[0], method, responseBody);
    if (!sanitized) return jsonError("collaboration_unavailable", 503);
    return NextResponse.json(sanitized, { status: upstream.status, headers: { "Cache-Control": "no-store" } });
  } catch (error) {
    return jsonError(isPrivateAPITimeout(error) ? "collaboration_unavailable" : "temporary_unavailable", 503);
  }
}

export async function readLiveBody(request: NextRequest, maximumBytes = 70_000): Promise<Record<string, unknown>> {
  const value = await readBoundedJSON<unknown>(request, maximumBytes);
  if (typeof value !== "object" || value === null || Array.isArray(value)) throw new Error("invalid body");
  return value as Record<string, unknown>;
}

export function validLiveId(value: string): boolean { return isLiveUUID(value); }
export function validIdempotencyKey(request: NextRequest): boolean {
  const value = request.headers.get("idempotency-key")?.trim() ?? "";
  return value.length >= 16 && value.length <= 255 && /^[\x21-\x7e]+$/u.test(value);
}
