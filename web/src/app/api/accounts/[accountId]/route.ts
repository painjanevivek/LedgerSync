import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { proxyPrivateGET } from "@/lib/private-api";
import { jsonError } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";

export async function GET(request: NextRequest, context: { params: Promise<{ accountId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  const { accountId } = await context.params;
  if (!accountId || accountId.length > 128) return jsonError("not_found", 404);
  return proxyPrivateGET(request, session, `/api/accounts/${encodeURIComponent(accountId)}`, []);
}
