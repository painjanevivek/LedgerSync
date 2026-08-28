import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { isFundingEventID } from "@/lib/api/funding";
import { authorizeFundingMutation, isFundingDenial } from "@/lib/funding-boundary";
import { proxyFundingMutation } from "@/lib/funding-mutation";
import { jsonError } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";

export async function POST(request: NextRequest, context: { params: Promise<{ fundingEventId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = authorizeFundingMutation(request, session, "funding:write", false, false);
  if (isFundingDenial(authorization)) return authorization;
  if (!session) return jsonError("unauthorized", 401);
  const { fundingEventId } = await context.params;
  if (!isFundingEventID(fundingEventId)) return jsonError("not_found", 404);
  return proxyFundingMutation(session, `/api/funding-events/${encodeURIComponent(fundingEventId)}/post`, undefined);
}
