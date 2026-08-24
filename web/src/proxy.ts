import { NextRequest, NextResponse } from "next/server";

import { addSecurityHeaders, contentSecurityPolicy, hasValidHost } from "@/lib/security";

export function proxy(_request: NextRequest) {
  const nonce = crypto.randomUUID().replaceAll("-", "");
  if (!hasValidHost(_request)) {
    return addSecurityHeaders(new NextResponse(null, { status: 421, headers: { "Cache-Control": "no-store" } }), nonce);
  }
  const requestHeaders = new Headers(_request.headers);
  requestHeaders.set("x-nonce", nonce);
  requestHeaders.set("Content-Security-Policy", contentSecurityPolicy(nonce));
  return addSecurityHeaders(NextResponse.next({ request: { headers: requestHeaders } }), nonce);
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
