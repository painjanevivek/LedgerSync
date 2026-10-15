import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { sanitizeUIPreference } from "@/lib/api/ui-preferences";
import { privateAPIContext } from "@/lib/private-api";
import { hasValidCSRF, jsonError, readBoundedJSON } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";

async function upstreamPreference(request: NextRequest, session: NonNullable<ReturnType<typeof readSession>>, method: "GET" | "PATCH", body?: string) {
  try {
    const connection = await privateAPIContext(session, request.headers.get("x-request-id") ?? undefined);
    const upstream = await fetch(`${connection.apiURL}/api/ui/preferences`, {
      method,
      headers: { ...connection.headers, ...(body ? { "Content-Type": "application/json" } : {}) },
      body,
      cache: "no-store",
      signal: AbortSignal.timeout(3_000),
    });
    const payload = await upstream.json().catch(() => null);
    if (!upstream.ok) return jsonError(upstream.status === 409 ? "preference_version_conflict" : upstream.status === 403 ? "forbidden" : "temporary_unavailable", upstream.status === 409 ? 409 : upstream.status === 403 ? 403 : 503);
    const preference = sanitizeUIPreference(payload);
    if (!preference) return jsonError("temporary_unavailable", 503);
    return NextResponse.json(preference, { headers: { "Cache-Control": "no-store" } });
  } catch {
    return jsonError("temporary_unavailable", 503);
  }
}

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  if (request.nextUrl.searchParams.size > 0) return jsonError("invalid_request", 400);
  return upstreamPreference(request, session, "GET");
}

export async function PATCH(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  if (!hasValidCSRF(request, session)) return jsonError("csrf_failed", 403);
  let input: unknown;
  try { input = await readBoundedJSON<unknown>(request, 1024); } catch { return jsonError("invalid_request", 400); }
  if (!input || typeof input !== "object" || Array.isArray(input)) return jsonError("invalid_request", 400);
  const record = input as Record<string, unknown>;
  if (Object.keys(record).length !== 2 || (record.experience_mode !== "simple" && record.experience_mode !== "expert") || typeof record.expected_version !== "string" || !/^(?:0|[1-9][0-9]{0,18})$/.test(record.expected_version)) return jsonError("invalid_request", 400);
  return upstreamPreference(request, session, "PATCH", JSON.stringify(record));
}
