import assert from "node:assert/strict";
import { createHmac } from "node:crypto";
import test from "node:test";

import { NextRequest, NextResponse } from "next/server";

import { addSecurityHeaders, hasValidCSRF } from "../../src/lib/security";
import { createSession, readSession, type Session } from "../../src/lib/session";
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
  const good = new NextRequest("https://ledger.example/api/transfers", { method: "POST", headers: { origin: "https://ledger.example", "x-csrf-token": session.csrfToken } });
  const crossOrigin = new NextRequest("https://ledger.example/api/transfers", { method: "POST", headers: { origin: "https://attacker.example", "x-csrf-token": session.csrfToken } });
  assert.equal(hasValidCSRF(good, session), true);
  assert.equal(hasValidCSRF(crossOrigin, session), false);
});

test("security headers deny framing, objects, and unexpected browser permissions", () => {
  const response = addSecurityHeaders(NextResponse.json({ ok: true }));
  assert.match(response.headers.get("Content-Security-Policy") ?? "", /frame-ancestors 'none'/);
  assert.equal(response.headers.get("X-Frame-Options"), "DENY");
  assert.equal(response.headers.get("X-Content-Type-Options"), "nosniff");
  assert.equal(response.headers.get("Permissions-Policy"), "camera=(), geolocation=(), microphone=()");
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
