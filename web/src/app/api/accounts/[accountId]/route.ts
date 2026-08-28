import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { accountMutationMaximumBytes, parseUpdateAccountRequest } from "@/lib/api/accounts";
import { proxyAccountMutation } from "@/lib/account-mutation";
import { authorizeAccountMutation, isAccountMutationDenial } from "@/lib/account-mutation-boundary";
import { proxyPrivateGET } from "@/lib/private-api";
import { jsonError, readBoundedJSON } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";

export async function GET(request: NextRequest, context: { params: Promise<{ accountId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  const { accountId } = await context.params;
  if (!accountId || accountId.length > 128) return jsonError("not_found", 404);
  return proxyPrivateGET(request, session, `/api/accounts/${encodeURIComponent(accountId)}`, []);
}

export async function PATCH(request: NextRequest, context: { params: Promise<{ accountId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = authorizeAccountMutation(request, session, "PATCH");
  if (isAccountMutationDenial(authorization)) return authorization;
  if (!session) return jsonError("unauthorized", 401);

  const { accountId } = await context.params;
  if (!accountId || accountId.length > 128) return jsonError("not_found", 404);
  let body: ReturnType<typeof parseUpdateAccountRequest>;
  try {
    body = parseUpdateAccountRequest(await readBoundedJSON<unknown>(request, accountMutationMaximumBytes));
  } catch {
    return jsonError("validation_failed", 400);
  }
  return proxyAccountMutation(session, "PATCH", `/api/accounts/${encodeURIComponent(accountId)}`, body, authorization.idempotencyKey);
}
