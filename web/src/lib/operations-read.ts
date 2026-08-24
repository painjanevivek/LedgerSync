import { NextRequest, NextResponse } from "next/server";

import type { SanitizedOperationsResponse } from "@/lib/api/operations";
import { sanitizeOperationsBody } from "@/lib/api/operations";
import type { RateLimitStore } from "@/lib/rate-limit";
import { hasValidHost, jsonError } from "@/lib/security";
import type { Session } from "@/lib/session";
import { isPrivateAPITimeout, privateReadTimeoutMilliseconds } from "@/lib/upstream-outcome";

const maximumOperationsResponseBytes = 262_144;

export type OperationsReadAuthorization = Readonly<{ session: Session }>;

export async function authorizeOperationsRead(request: NextRequest, session: Session | null, scope: "local:read" | "events:read" | "developer:read", rateLimit: RateLimitStore): Promise<OperationsReadAuthorization | NextResponse> {
  if (request.method !== "GET") {
    const response = jsonError("method_not_allowed", 405);
    response.headers.set("Allow", "GET");
    return response;
  }
  if (!session) return jsonError("unauthorized", 401);
  if (!session.scopes?.includes(scope)) return jsonError("forbidden", 403);
  if (!hasValidHost(request)) return jsonError("invalid_request", 400);
  const decision = await rateLimit.consume(`operations:${scope}:${session.tenantId}:${session.subjectId}`, 60, 60);
  if (!decision.allowed) {
    const response = jsonError("rate_limited", 429);
    response.headers.set("Retry-After", String(decision.retryAfterSeconds));
    return response;
  }
  return { session };
}

export function isOperationsReadDenial(value: OperationsReadAuthorization | NextResponse): value is NextResponse {
  return value instanceof NextResponse;
}

export function strictOperationsQuery(request: NextRequest, allowed: readonly string[]): URLSearchParams | NextResponse {
  const permitted = new Set(allowed);
  const query = new URLSearchParams();
  for (const [key] of request.nextUrl.searchParams) {
    if (!permitted.has(key) || request.nextUrl.searchParams.getAll(key).length !== 1) return jsonError("validation_failed", 400);
  }
  for (const key of allowed) {
    const raw = request.nextUrl.searchParams.get(key);
    if (raw === null || raw === "") continue;
    const value = raw.trim();
    if (!value || value.length > (key === "cursor" ? 2_048 : 256)) return jsonError("validation_failed", 400);
    if (key === "limit" && (!/^[1-9][0-9]{0,2}$/.test(value) || Number(value) > 100)) return jsonError("validation_failed", 400);
    if (key === "state" && !["pending", "retrying", "published", "dead"].includes(value)) return jsonError("validation_failed", 400);
    if ((key === "from" || key === "to") && Number.isNaN(Date.parse(value))) return jsonError("validation_failed", 400);
    if (key === "eventType" && !/^[A-Za-z0-9][A-Za-z0-9._:-]*$/.test(value)) return jsonError("validation_failed", 400);
    if ((key === "relatedId" || key === "correlationId") && !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value)) return jsonError("validation_failed", 400);
    query.set(key, value);
  }
  const from = query.get("from");
  const to = query.get("to");
  if (from && to && Date.parse(from) > Date.parse(to)) return jsonError("validation_failed", 400);
  return query;
}

function safeHeaders(upstream: Headers): Record<string, string> {
  const result: Record<string, string> = { "Cache-Control": "no-store" };
  const requestID = upstream.get("x-request-id");
  if (requestID && /^[A-Za-z0-9._:-]{1,128}$/.test(requestID)) result["X-Request-ID"] = requestID;
  const retryAfter = upstream.get("retry-after");
  if (retryAfter && /^[0-9]{1,6}$/.test(retryAfter)) result["Retry-After"] = retryAfter;
  return result;
}

export async function readBoundedOperationsResponse(response: Response): Promise<string> {
  const advertised = response.headers.get("content-length");
  if (advertised !== null) {
    const bytes = Number(advertised);
    if (!Number.isSafeInteger(bytes) || bytes < 0 || bytes > maximumOperationsResponseBytes) throw new Error("response_too_large");
  }
  if (!response.body) return "";
  const reader = response.body.getReader();
  const decoder = new TextDecoder("utf-8", { fatal: true });
  let received = 0;
  let text = "";
  try {
    while (true) {
      const chunk = await reader.read();
      if (chunk.done) break;
      received += chunk.value.byteLength;
      if (received > maximumOperationsResponseBytes) throw new Error("response_too_large");
      text += decoder.decode(chunk.value, { stream: true });
    }
    return text + decoder.decode();
  } finally {
    reader.releaseLock();
  }
}

export async function proxyOperationsGET(
  session: Session,
  path: string,
  query: URLSearchParams,
  sanitizer: (status: number, value: unknown) => SanitizedOperationsResponse,
): Promise<NextResponse> {
  let connection: Readonly<{ apiURL: string; headers: Record<string, string> }>;
  try {
    const { privateAPIContext } = await import("@/lib/private-api");
    connection = await privateAPIContext(session);
  }
  catch { return jsonError("temporary_unavailable", 503); }
  try {
    const suffix = query.size ? `?${query}` : "";
    const upstream = await fetch(`${connection.apiURL}${path}${suffix}`, {
      headers: connection.headers,
      cache: "no-store",
      signal: AbortSignal.timeout(privateReadTimeoutMilliseconds),
    });
    const raw = await readBoundedOperationsResponse(upstream);
    const sanitized = sanitizeOperationsBody(upstream.status, raw, sanitizer);
    return NextResponse.json(sanitized.body, { status: sanitized.status, headers: safeHeaders(upstream.headers) });
  } catch (error) {
    return jsonError(isPrivateAPITimeout(error) ? "upstream_timeout" : "temporary_unavailable", isPrivateAPITimeout(error) ? 504 : 503);
  }
}
