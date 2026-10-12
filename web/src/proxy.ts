import { NextRequest, NextResponse } from "next/server";

import { addSecurityHeaders, contentSecurityPolicy, hasValidHost } from "@/lib/security";
import { createSession, resolveOpaqueSession, sessionCookieName } from "@/lib/session";

export async function proxy(_request: NextRequest) {
  const nonce = crypto.randomUUID().replaceAll("-", "");
  if (!hasValidHost(_request)) {
    return addSecurityHeaders(new NextResponse(null, { status: 421, headers: { "Cache-Control": "no-store" } }), nonce);
  }
  const requestHeaders = new Headers(_request.headers);
  requestHeaders.delete("x-ledgersync-session-handle");
  const browserSession = _request.cookies.get(sessionCookieName)?.value;
  const publicPage = ["/", "/welcome", "/sign-in"].includes(_request.nextUrl.pathname);
  if (browserSession && !publicPage) {
    let resolved = null;
    try {
      resolved = await resolveOpaqueSession(browserSession);
    } catch {
      resolved = null;
    }
    const requestCookies = _request.cookies.getAll()
      .filter((cookie) => cookie.name !== sessionCookieName)
      .map((cookie) => `${cookie.name}=${cookie.value}`);
    if (resolved) {
      requestCookies.push(`${sessionCookieName}=${createSession(resolved)}`);
      requestHeaders.set("x-ledgersync-session-handle", browserSession);
    }
    if (requestCookies.length > 0) requestHeaders.set("cookie", requestCookies.join("; "));
    else requestHeaders.delete("cookie");
  }
  requestHeaders.set("x-nonce", nonce);
  requestHeaders.set("Content-Security-Policy", contentSecurityPolicy(nonce));
  const response = addSecurityHeaders(NextResponse.next({ request: { headers: requestHeaders } }), nonce);
  response.headers.set("Cache-Control", "private, no-store");
  if (!publicPage) response.headers.set("X-Robots-Tag", "noindex, nofollow");
  return response;
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
