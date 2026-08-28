import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

import { completeAuthorization, transactionCookieName } from "@/lib/oidc";
import { createSession, sessionCookie } from "@/lib/session";
import { jsonError } from "@/lib/security";

export async function GET(request: NextRequest) {
  const code = request.nextUrl.searchParams.get("code");
  const state = request.nextUrl.searchParams.get("state");
  if (!code || !state) return jsonError("authentication_failed", 400);
  try {
    const { identity, returnTo } = await completeAuthorization(
      code,
      state,
      (await cookies()).get(transactionCookieName)?.value,
    );
    const response = NextResponse.redirect(new URL(returnTo, request.url));
    response.cookies.set(
      sessionCookie(
        createSession({ ...identity, csrfToken: crypto.randomUUID() }),
      ),
    );
    response.cookies.delete(transactionCookieName);
    return response;
  } catch {
    return jsonError("authentication_failed", 401);
  }
}
