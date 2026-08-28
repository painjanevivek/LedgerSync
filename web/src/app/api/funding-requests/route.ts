import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { fundingMutationMaximumBytes, toPrivateFundingRequest, type CreateFundingInput } from "@/lib/api/funding";
import { authorizeFundingMutation, isFundingDenial } from "@/lib/funding-boundary";
import { proxyFundingMutation } from "@/lib/funding-mutation";
import { jsonError, readBoundedJSON } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";

export async function POST(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = authorizeFundingMutation(request, session, "funding:write", true, true);
  if (isFundingDenial(authorization)) return authorization;
  if (!session || !authorization.idempotencyKey) return jsonError("unauthorized", 401);
  try {
    const body = toPrivateFundingRequest(await readBoundedJSON<CreateFundingInput>(request, fundingMutationMaximumBytes));
    return proxyFundingMutation(session, "/api/funding-requests", body, authorization.idempotencyKey);
  } catch {
    return jsonError("validation_failed", 400);
  }
}
