import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { proxyPrivateGET } from "@/lib/private-api";
import { readSession, sessionCookieName } from "@/lib/session";
import { jsonError } from "@/lib/security";
import { canonicalUUID } from "@/lib/canonical-uuid";

export async function GET(request: NextRequest, context: { params: Promise<{ transferId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  const transferId = canonicalUUID((await context.params).transferId);
  if (!transferId) return jsonError("validation_failed", 400);
  return proxyPrivateGET(request, session, `/api/transfers/${encodeURIComponent(transferId)}`);
}
