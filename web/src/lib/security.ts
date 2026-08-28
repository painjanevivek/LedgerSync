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

export function readPublicOrigin(environment: Readonly<Record<string, string | undefined>> = process.env): URL {
  const configuredOrigin = environment.LEDGERSYNC_PUBLIC_ORIGIN?.trim();
  if (!configuredOrigin) throw new Error("LEDGERSYNC_PUBLIC_ORIGIN is required");
  let origin: URL;
  try {
    origin = new URL(configuredOrigin);
  } catch {
    throw new Error("LEDGERSYNC_PUBLIC_ORIGIN must be a valid HTTP(S) origin");
  }
  if (
    (origin.protocol !== "http:" && origin.protocol !== "https:")
    || origin.username
    || origin.password
    || origin.pathname !== "/"
    || origin.search
    || origin.hash
  ) {
    throw new Error("LEDGERSYNC_PUBLIC_ORIGIN must be a valid HTTP(S) origin");
  }
  return origin;
}

export function contentSecurityPolicy(nonce?: string): string {
  const scriptSource = nonce ? `script-src 'self' 'nonce-${nonce}'` : "script-src 'self'";
  const styleSource = nonce ? `style-src 'self' 'nonce-${nonce}'` : "style-src 'self'";
  return `default-src 'self'; ${scriptSource}; ${styleSource}; base-uri 'self'; frame-ancestors 'none'; form-action 'self'; object-src 'none'`;
}

export function addSecurityHeaders(response: NextResponse, nonce?: string): NextResponse {
  response.headers.set("Content-Security-Policy", contentSecurityPolicy(nonce));
  for (const [name, value] of Object.entries(securityHeaders)) response.headers.set(name, value);
  const deploymentEnvironment = (process.env.LEDGERSYNC_DEPLOYMENT_ENV ?? "development").trim().toLowerCase();
  response.headers.set("X-LedgerSync-Mode", deploymentEnvironment === "production" || deploymentEnvironment === "prod" ? "production" : "sandbox");
  if (deploymentEnvironment === "production" || deploymentEnvironment === "prod") {
    response.headers.set("Strict-Transport-Security", "max-age=31536000; includeSubDomains");
  }
  return response;
}

export function hasValidHost(request: NextRequest): boolean {
  try {
    const expected = readPublicOrigin();
    const host = request.headers.get("host");
    if (!host) return false;
    const supplied = new URL(`${expected.protocol}//${host}`);
    return supplied.username === "" && supplied.password === "" && supplied.pathname === "/" && supplied.search === "" && supplied.hash === "" && supplied.origin === expected.origin;
  } catch {
    return false;
  }
}

// Cookie-authenticated mutations must originate from this fixed public origin.
export function hasSameOrigin(request: NextRequest): boolean {
  const origin = request.headers.get("origin");
  if (origin === null || !hasValidHost(request)) return false;
  try {
    return origin === readPublicOrigin().origin;
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
