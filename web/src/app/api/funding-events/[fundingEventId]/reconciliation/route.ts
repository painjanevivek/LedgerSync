import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { isFundingEventID } from "@/lib/api/funding";
import { proxyPrivateGET } from "@/lib/private-api";
import { jsonError } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";

export async function GET(request: NextRequest, context: { params: Promise<{ fundingEventId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  if (!session.scopes?.includes("funding:read")) return jsonError("forbidden", 403);
  const { fundingEventId } = await context.params;
  if (!isFundingEventID(fundingEventId)) return jsonError("not_found", 404);
  return proxyPrivateGET(request, session, `/api/funding-events/${encodeURIComponent(fundingEventId)}/reconciliation`, []);
}
