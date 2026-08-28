import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { proxyPrivateGET } from "@/lib/private-api";
import { jsonError } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  if (!session.scopes?.includes("funding:read")) return jsonError("forbidden", 403);
  return proxyPrivateGET(request, session, "/api/funding-events", ["cursor", "limit", "status"]);
}
