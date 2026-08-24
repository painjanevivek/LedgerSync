import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { proxyPrivateGET } from "@/lib/private-api";
import { readSession, sessionCookieName } from "@/lib/session";
import { jsonError } from "@/lib/security";

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  return proxyPrivateGET(request, session, "/api/reconciliation/runs", ["cursor", "limit"]);
}
