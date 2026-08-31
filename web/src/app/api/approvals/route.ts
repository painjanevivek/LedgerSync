import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { proxyPrivateGET } from "@/lib/private-api";
import { jsonError } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";

const approvalFilters = [
  "domain",
  "status",
  "requester",
  "age",
  "requested_after",
  "requested_before",
  "actionable_by_me",
  "cursor",
  "limit",
] as const;

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  const canApprove =
    session.scopes?.includes("funding:approve") ||
    session.scopes?.includes("corrections:approve");
  if (!canApprove) return jsonError("forbidden", 403);
  return proxyPrivateGET(request, session, "/api/approvals", approvalFilters);
}
