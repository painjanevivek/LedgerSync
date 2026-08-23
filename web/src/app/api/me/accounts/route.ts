import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { sessionCookieName, readSession } from "@/lib/session";
import { jsonError } from "@/lib/security";
import { proxyPrivateGET } from "@/lib/private-api";

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  return proxyPrivateGET(request, session, "/api/me/accounts", ["cursor", "limit", "q", "status", "category"]);
}
