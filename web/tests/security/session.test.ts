import assert from "node:assert/strict";
import { createHmac } from "node:crypto";
import test from "node:test";

import { NextRequest, NextResponse } from "next/server";

import { addSecurityHeaders, contentSecurityPolicy, hasValidCSRF } from "../../src/lib/security";
import { createSession, readSession, sessionCookie, type Session } from "../../src/lib/session";
import { readTransaction } from "../../src/lib/oidc";

process.env.LEDGERSYNC_WEB_SESSION_SECRET = "phase-five-test-secret-that-is-long-enough";

const session: Session = { subjectId: "operator-a", tenantId: "tenant-a", csrfToken: "csrf-token", expiresAt: Date.now() + 60_000 };

test("signed sessions reject tampering and expiry", () => {
  const encoded = createSession(session);
  assert.deepEqual(readSession(encoded), { ...session, consistencyRequirements: undefined, roles: undefined, scopes: undefined });
  assert.equal(readSession(`${encoded}x`), null);
  assert.equal(readSession(createSession({ ...session, expiresAt: Date.now() - 1 })), null);
});

test("cookie-authenticated mutations require same-origin CSRF", () => {
  const previousOrigin = process.env.LEDGERSYNC_PUBLIC_ORIGIN;
  const previousDeployment = process.env.LEDGERSYNC_DEPLOYMENT_ENV;
  try {
    process.env.LEDGERSYNC_PUBLIC_ORIGIN = "https://ledger.example";
    process.env.LEDGERSYNC_DEPLOYMENT_ENV = "production";
    const good = new NextRequest("http://internal-web/api/transfers", { method: "POST", headers: { origin: "https://ledger.example", "x-csrf-token": session.csrfToken } });
    const crossOrigin = new NextRequest("http://internal-web/api/transfers", { method: "POST", headers: { origin: "https://attacker.example", "x-csrf-token": session.csrfToken } });
    assert.equal(hasValidCSRF(good, session), true);
    assert.equal(hasValidCSRF(crossOrigin, session), false);
    delete process.env.LEDGERSYNC_PUBLIC_ORIGIN;
    assert.equal(hasValidCSRF(good, session), false);
  } finally {
    if (previousOrigin === undefined) delete process.env.LEDGERSYNC_PUBLIC_ORIGIN; else process.env.LEDGERSYNC_PUBLIC_ORIGIN = previousOrigin;
    if (previousDeployment === undefined) delete process.env.LEDGERSYNC_DEPLOYMENT_ENV; else process.env.LEDGERSYNC_DEPLOYMENT_ENV = previousDeployment;
  }
});

test("insecure cookies are explicit-local only and cannot weaken production", () => {
  const previousDeployment = process.env.LEDGERSYNC_DEPLOYMENT_ENV;
  const previousSecure = process.env.LEDGERSYNC_COOKIE_SECURE;
  try {
    process.env.LEDGERSYNC_COOKIE_SECURE = "false";
    process.env.LEDGERSYNC_DEPLOYMENT_ENV = "development";
    assert.equal(sessionCookie("value").secure, false);
    process.env.LEDGERSYNC_DEPLOYMENT_ENV = "production";
    assert.equal(sessionCookie("value").secure, true);
  } finally {
    if (previousDeployment === undefined) delete process.env.LEDGERSYNC_DEPLOYMENT_ENV; else process.env.LEDGERSYNC_DEPLOYMENT_ENV = previousDeployment;
    if (previousSecure === undefined) delete process.env.LEDGERSYNC_COOKIE_SECURE; else process.env.LEDGERSYNC_COOKIE_SECURE = previousSecure;
  }
});

test("security headers deny unsafe browser behavior and enable HSTS only for production HTTPS deployments", () => {
  const previousDeployment = process.env.LEDGERSYNC_DEPLOYMENT_ENV;
  try {
    process.env.LEDGERSYNC_DEPLOYMENT_ENV = "development";
    const localResponse = addSecurityHeaders(NextResponse.json({ ok: true }));
    assert.match(localResponse.headers.get("Content-Security-Policy") ?? "", /frame-ancestors 'none'/);
    assert.equal(localResponse.headers.get("X-Frame-Options"), "DENY");
    assert.equal(localResponse.headers.get("X-Content-Type-Options"), "nosniff");
    assert.equal(localResponse.headers.get("Permissions-Policy"), "camera=(), geolocation=(), microphone=()");
    assert.equal(localResponse.headers.get("Strict-Transport-Security"), null);

    process.env.LEDGERSYNC_DEPLOYMENT_ENV = "production";
    const productionResponse = addSecurityHeaders(NextResponse.json({ ok: true }));
    assert.equal(productionResponse.headers.get("Strict-Transport-Security"), "max-age=31536000; includeSubDomains");
  } finally {
    if (previousDeployment === undefined) delete process.env.LEDGERSYNC_DEPLOYMENT_ENV; else process.env.LEDGERSYNC_DEPLOYMENT_ENV = previousDeployment;
  }
});

test("the production CSP uses a per-request nonce instead of unsafe inline scripts", () => {
  const csp = contentSecurityPolicy("test-nonce");
  assert.match(csp, /script-src 'self' 'nonce-test-nonce'/);
  assert.doesNotMatch(csp, /unsafe-inline/);
});

test("OIDC transaction cookies reject tampering and expired authorizations", () => {
  const payload = Buffer.from(JSON.stringify({ state: "state", codeVerifier: "verifier", nonce: "nonce", expiresAt: Date.now() + 60_000 })).toString("base64url");
  const signature = createHmac("sha256", process.env.LEDGERSYNC_WEB_SESSION_SECRET!).update(payload).digest("base64url");
  assert.equal(readTransaction(`${payload}.${signature}`)?.state, "state");
  assert.equal(readTransaction(`${payload}.${signature}x`), null);
  const expiredPayload = Buffer.from(JSON.stringify({ state: "state", codeVerifier: "verifier", nonce: "nonce", expiresAt: Date.now() - 1 })).toString("base64url");
  const expiredSignature = createHmac("sha256", process.env.LEDGERSYNC_WEB_SESSION_SECRET!).update(expiredPayload).digest("base64url");
  assert.equal(readTransaction(`${expiredPayload}.${expiredSignature}`), null);
});
