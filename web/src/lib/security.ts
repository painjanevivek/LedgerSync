import { NextRequest, NextResponse } from "next/server";

import type { Session } from "@/lib/session";

export const securityHeaders: Record<string, string> = {
  "Cross-Origin-Opener-Policy": "same-origin",
  "Cross-Origin-Resource-Policy": "same-origin",
  "Referrer-Policy": "strict-origin-when-cross-origin",
  "X-Content-Type-Options": "nosniff",
  "X-Frame-Options": "DENY",
  "Permissions-Policy": "camera=(), geolocation=(), microphone=()",
};

export function contentSecurityPolicy(nonce?: string): string {
  const scriptSource = nonce ? `script-src 'self' 'nonce-${nonce}'` : "script-src 'self'";
  const styleSource = nonce ? `style-src 'self' 'nonce-${nonce}'` : "style-src 'self'";
  return `default-src 'self'; ${scriptSource}; ${styleSource}; base-uri 'self'; frame-ancestors 'none'; form-action 'self'; object-src 'none'`;
}

export function addSecurityHeaders(response: NextResponse, nonce?: string): NextResponse {
  response.headers.set("Content-Security-Policy", contentSecurityPolicy(nonce));
  for (const [name, value] of Object.entries(securityHeaders)) response.headers.set(name, value);
  const deploymentEnvironment = (process.env.LEDGERSYNC_DEPLOYMENT_ENV ?? "development").trim().toLowerCase();
  if (deploymentEnvironment === "production" || deploymentEnvironment === "prod") {
    response.headers.set("Strict-Transport-Security", "max-age=31536000; includeSubDomains");
  }
  return response;
}

// Cookie-authenticated mutations must originate from this same public origin.
export function hasSameOrigin(request: NextRequest): boolean {
  const origin = request.headers.get("origin");
  if (origin === null) return false;
  const configuredOrigin = process.env.LEDGERSYNC_PUBLIC_ORIGIN?.trim();
  if (!configuredOrigin) return false;
  try {
    const expected = new URL(configuredOrigin);
    return origin === expected.origin && request.headers.get("host") === expected.host;
  } catch {
    return false;
  }
}

export function hasValidCSRF(request: NextRequest, session: Session): boolean {
  const token = request.headers.get("x-csrf-token");
  if (!hasSameOrigin(request) || token === null || token.length !== session.csrfToken.length) return false;
  let difference = 0;
  for (let index = 0; index < token.length; index += 1) difference |= token.charCodeAt(index) ^ session.csrfToken.charCodeAt(index);
  return difference === 0;
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
