import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { accountMutationMaximumBytes, parseCreateAccountRequest } from "@/lib/api/accounts";
import { proxyAccountMutation } from "@/lib/account-mutation";
import { authorizeAccountMutation, isAccountMutationDenial } from "@/lib/account-mutation-boundary";
import { sessionCookieName, readSession } from "@/lib/session";
import { jsonError, readBoundedJSON } from "@/lib/security";
import { proxyPrivateGET } from "@/lib/private-api";

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  return proxyPrivateGET(request, session, "/api/me/accounts", ["cursor", "limit", "q", "status", "category"]);
}

export async function POST(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = authorizeAccountMutation(request, session, "POST");
  if (isAccountMutationDenial(authorization)) return authorization;
  if (!session) return jsonError("unauthorized", 401);

  let body: ReturnType<typeof parseCreateAccountRequest>;
  try {
    body = parseCreateAccountRequest(await readBoundedJSON<unknown>(request, accountMutationMaximumBytes));
  } catch {
    return jsonError("validation_failed", 400);
  }
  return proxyAccountMutation(session, "POST", "/api/accounts", body, authorization.idempotencyKey);
}
