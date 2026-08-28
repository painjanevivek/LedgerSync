import { NextRequest, NextResponse } from "next/server";

import { beginAuthorization, transactionCookie } from "@/lib/oidc";
import { jsonError } from "@/lib/security";

export async function GET(request: NextRequest) {
  try {
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
