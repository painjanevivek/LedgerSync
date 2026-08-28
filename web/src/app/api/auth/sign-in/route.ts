import { NextRequest, NextResponse } from "next/server";

import { createLocalSession, readLocalAccessConfiguration } from "@/lib/local-access";
import { beginAuthorization, safeReturnTo, transactionCookie } from "@/lib/oidc";
import { jsonError } from "@/lib/security";
import { createSession, sessionCookie } from "@/lib/session";

export async function GET(request: NextRequest) {
  try {
    const localAccess = readLocalAccessConfiguration();
    if (localAccess.enabled) {
      const returnTo = safeReturnTo(
        request.nextUrl.searchParams.get("return_to"),
      );
      const response = NextResponse.redirect(new URL(returnTo, request.url));
      response.cookies.set(
        sessionCookie(createSession(createLocalSession(localAccess))),
      );
      return response;
    }
    const authorization = await beginAuthorization({
      prompt:
        request.nextUrl.searchParams.get("prompt") === "login"
          ? "login"
          : undefined,
      returnTo: request.nextUrl.searchParams.get("return_to") ?? undefined,
    });
    const response = NextResponse.redirect(authorization.authorizationURL);
    response.cookies.set(transactionCookie(authorization.transactionCookie));
    return response;
  } catch {
    return jsonError("authentication_unavailable", 503);
  }
}
