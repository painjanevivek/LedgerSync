import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { proxyPrivateGET } from "@/lib/private-api";
import { readSession, sessionCookieName } from "@/lib/session";
import { jsonError } from "@/lib/security";

export async function GET(request: NextRequest, context: { params: Promise<{ transferId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  const { transferId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(transferId)) return jsonError("not_found", 404);
  return proxyPrivateGET(request, session, `/api/transfers/${encodeURIComponent(transferId)}`);
}
