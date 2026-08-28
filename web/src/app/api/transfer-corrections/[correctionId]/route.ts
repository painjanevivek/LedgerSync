import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { isCorrectionID } from "@/lib/api/corrections";
import { proxyPrivateGET } from "@/lib/private-api";
import { jsonError } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";

export async function GET(
  request: NextRequest,
  context: { params: Promise<{ correctionId: string }> },
) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  if (!session.scopes?.includes("corrections:read"))
    return jsonError("forbidden", 403);
  const { correctionId } = await context.params;
  if (!isCorrectionID(correctionId)) return jsonError("not_found", 404);
  return proxyPrivateGET(
    request,
    session,
    `/api/transfer-corrections/${encodeURIComponent(correctionId)}`,
  );
}
