import assert from "node:assert/strict";
import { createHmac } from "node:crypto";
import test from "node:test";

import { NextRequest, NextResponse } from "next/server";

import { createDemoSession } from "../../src/lib/demo";
import { addSecurityHeaders, contentSecurityPolicy, hasValidCSRF, hasValidHost, readPublicOrigin } from "../../src/lib/security";
import { createSession, readSession, sessionCookie, type Session } from "../../src/lib/session";
import { readTransaction, transactionCookie } from "../../src/lib/oidc";
import { proxy } from "../../src/proxy";

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
    const good = new NextRequest("https://ledger.example/api/transfers", { method: "POST", headers: { host: "ledger.example", origin: "https://ledger.example", "x-csrf-token": session.csrfToken } });
    const crossOrigin = new NextRequest("https://ledger.example/api/transfers", { method: "POST", headers: { host: "ledger.example", origin: "https://attacker.example", "x-csrf-token": session.csrfToken } });
    const reboundHost = new NextRequest("https://attacker.example/api/transfers", { method: "POST", headers: { host: "attacker.example", origin: "https://ledger.example", "x-csrf-token": session.csrfToken } });
    const reboundOriginAndHost = new NextRequest("https://attacker.example/api/transfers", { method: "POST", headers: { host: "attacker.example", origin: "https://attacker.example", "x-csrf-token": session.csrfToken } });
    assert.equal(hasValidCSRF(good, session), true);
    assert.equal(hasValidCSRF(crossOrigin, session), false);
    assert.equal(hasValidCSRF(reboundHost, session), false);
    assert.equal(hasValidCSRF(reboundOriginAndHost, session), false);
    assert.equal(hasValidHost(reboundOriginAndHost), false);
    delete process.env.LEDGERSYNC_PUBLIC_ORIGIN;
    assert.equal(hasValidCSRF(good, session), false);
  } finally {
    if (previousOrigin === undefined) delete process.env.LEDGERSYNC_PUBLIC_ORIGIN; else process.env.LEDGERSYNC_PUBLIC_ORIGIN = previousOrigin;
    if (previousDeployment === undefined) delete process.env.LEDGERSYNC_DEPLOYMENT_ENV; else process.env.LEDGERSYNC_DEPLOYMENT_ENV = previousDeployment;
  }
});

test("signed sessions preserve the complete bounded operator scope set", () => {
  const demoSession = createDemoSession({ enabled: true, environment: "development", subjectId: "operator-a", tenantId: "tenant-a" });
  assert.equal(demoSession.scopes?.length, 17);
  assert.deepEqual(readSession(createSession(demoSession))?.scopes, demoSession.scopes);
  assert.equal(readSession(createSession({ ...session, scopes: Array.from({ length: 33 }, (_, index) => `scope:${index}`) }))?.scopes, undefined);
});

test("public origin configuration is fixed and proxy rejects DNS-rebinding hosts", () => {
  const previousOrigin = process.env.LEDGERSYNC_PUBLIC_ORIGIN;
  try {
    delete process.env.LEDGERSYNC_PUBLIC_ORIGIN;
    assert.throws(() => readPublicOrigin(), /PUBLIC_ORIGIN is required/);
    assert.throws(() => sessionCookie("session"), /PUBLIC_ORIGIN is required/);
    assert.throws(() => transactionCookie("transaction"), /PUBLIC_ORIGIN is required/);
    process.env.LEDGERSYNC_PUBLIC_ORIGIN = "http://127.0.0.1:3000";
    const accepted = new NextRequest("http://127.0.0.1:3000/api/session", { headers: { host: "127.0.0.1:3000" } });
    const rebound = new NextRequest("http://attacker.example:3000/api/session", { headers: { host: "attacker.example:3000" } });
    assert.equal(hasValidHost(accepted), true);
    assert.equal(proxy(accepted).status, 200);
    const rejected = proxy(rebound);
    assert.equal(rejected.status, 421);
    assert.equal(rejected.headers.get("Cache-Control"), "no-store");
  } finally {
    if (previousOrigin === undefined) delete process.env.LEDGERSYNC_PUBLIC_ORIGIN; else process.env.LEDGERSYNC_PUBLIC_ORIGIN = previousOrigin;
  }
});

test("session signatures verify the exact encoded payload regardless of property insertion order", () => {
  const reordered: Session = { scopes: ["accounts:read"], expiresAt: Date.now() + 60_000, tenantId: "tenant-a", csrfToken: "csrf", subjectId: "operator-a", roles: ["tenant:operator"] };
  assert.deepEqual(readSession(createSession(reordered)), { ...reordered, consistencyRequirements: undefined });
});

test("insecure cookies are explicit-local only and cannot weaken production", () => {
  const previousDeployment = process.env.LEDGERSYNC_DEPLOYMENT_ENV;
  const previousSecure = process.env.LEDGERSYNC_COOKIE_SECURE;
  const previousOrigin = process.env.LEDGERSYNC_PUBLIC_ORIGIN;
  try {
    process.env.LEDGERSYNC_PUBLIC_ORIGIN = "http://127.0.0.1:3000";
    process.env.LEDGERSYNC_COOKIE_SECURE = "false";
    process.env.LEDGERSYNC_DEPLOYMENT_ENV = "development";
    assert.equal(sessionCookie("value").secure, false);
    process.env.LEDGERSYNC_DEPLOYMENT_ENV = "production";
    assert.equal(sessionCookie("value").secure, true);
  } finally {
    if (previousDeployment === undefined) delete process.env.LEDGERSYNC_DEPLOYMENT_ENV; else process.env.LEDGERSYNC_DEPLOYMENT_ENV = previousDeployment;
    if (previousSecure === undefined) delete process.env.LEDGERSYNC_COOKIE_SECURE; else process.env.LEDGERSYNC_COOKIE_SECURE = previousSecure;
    if (previousOrigin === undefined) delete process.env.LEDGERSYNC_PUBLIC_ORIGIN; else process.env.LEDGERSYNC_PUBLIC_ORIGIN = previousOrigin;
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
