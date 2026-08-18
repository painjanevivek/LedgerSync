import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { readSession, sessionCookieName } from "@/lib/session";
import { hasValidCSRF, jsonError } from "@/lib/security";

export async function POST(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  if (!hasValidCSRF(request, session)) return jsonError("csrf_failed", 403);
  const response = new NextResponse(null, { status: 204, headers: { "Cache-Control": "no-store" } });
  response.cookies.delete(sessionCookieName);
  return response;
}
