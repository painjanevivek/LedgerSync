import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { sessionCookieName, readSession } from "@/lib/session";
import { jsonError } from "@/lib/security";
import { proxyPrivateGET } from "@/lib/private-api";
import { isAccountId, parseAccountHistoryBFFSearchParams } from "@/lib/page-query/accounts";

export async function GET(request: NextRequest, context: { params: Promise<{ accountId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  const { accountId } = await context.params;
  if (!isAccountId(accountId)) return jsonError("not_found", 404);
  if (!parseAccountHistoryBFFSearchParams(request.nextUrl.searchParams).ok) return jsonError("validation_failed", 400);
  return proxyPrivateGET(request, session, `/api/accounts/${encodeURIComponent(accountId)}/transactions`, ["cursor", "limit"]);
}
