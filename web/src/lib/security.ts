import { NextRequest, NextResponse } from "next/server";

import type { Session } from "@/lib/session";

export const securityHeaders: Record<string, string> = {
  "Content-Security-Policy": "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'; object-src 'none'",
  "Cross-Origin-Opener-Policy": "same-origin",
  "Cross-Origin-Resource-Policy": "same-origin",
  "Referrer-Policy": "strict-origin-when-cross-origin",
  "X-Content-Type-Options": "nosniff",
  "X-Frame-Options": "DENY",
  "Permissions-Policy": "camera=(), geolocation=(), microphone=()",
};

export function addSecurityHeaders(response: NextResponse): NextResponse {
  for (const [name, value] of Object.entries(securityHeaders)) response.headers.set(name, value);
  return response;
}

// Cookie-authenticated mutations must originate from this same public origin.
export function hasSameOrigin(request: NextRequest): boolean {
  const origin = request.headers.get("origin");
  return origin !== null && origin === request.nextUrl.origin;
}

export function hasValidCSRF(request: NextRequest, session: Session): boolean {
  const token = request.headers.get("x-csrf-token");
  return hasSameOrigin(request) && token !== null && token.length === session.csrfToken.length && token === session.csrfToken;
}

export async function readBoundedJSON<T>(request: NextRequest, maximumBytes = 16_384): Promise<T> {
  const advertisedSize = Number(request.headers.get("content-length") ?? "0");
  if (!Number.isInteger(advertisedSize) || advertisedSize < 1 || advertisedSize > maximumBytes) {
    throw new Error("request body is outside the permitted size");
  }
  const body = await request.text();
  if (new TextEncoder().encode(body).byteLength > maximumBytes) {
    throw new Error("request body is outside the permitted size");
  }
  return JSON.parse(body) as T;
}

export function jsonError(message: string, status: number): NextResponse {
  return NextResponse.json({ error: { code: message } }, { status, headers: { "Cache-Control": "no-store" } });
}
