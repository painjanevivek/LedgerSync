import { NextResponse } from "next/server";

import { beginAuthorization, transactionCookie } from "@/lib/oidc";
import { jsonError } from "@/lib/security";

export async function GET() {
  try {
    const authorization = await beginAuthorization();
    const response = NextResponse.redirect(authorization.authorizationURL);
    response.cookies.set(transactionCookie(authorization.transactionCookie));
    return response;
  } catch { return jsonError("authentication_unavailable", 503); }
}
