import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { NextRequest } from "next/server";

import {
  isValidReconciliationIdempotencyKey,
  parseReconciliationRunRequest,
  reconciliationMutationMaximumBytes,
  sanitizeReconciliationUpstream,
  sanitizeReconciliationUpstreamBody,
} from "../../src/lib/api/reconciliation";
import type { RateLimitStore } from "../../src/lib/rate-limit";
import {
  authorizeReconciliationMutation,
  isReconciliationMutationDenial,
  reconciliationDispatchError,
  reconciliationPrivateHeaders,
  reconciliationResponseHeaders,
} from "../../src/lib/reconciliation-mutation-boundary";
import { readBoundedJSON } from "../../src/lib/security";
import { createSession, readSession, type Session } from "../../src/lib/session";

const publicOrigin = "http://127.0.0.1:3000";
const validKey = "reconcile-command-0001";
const session: Session = {
  subjectId: "operator-a",
  tenantId: "tenant-a",
  csrfToken: "csrf-reconciliation-command",
  expiresAt: Date.now() + 60_000,
  scopes: ["reconciliation:read", "reconciliation:write"],
};
const allow: RateLimitStore = { consume: async () => ({ allowed: true, retryAfterSeconds: 0 }) };

function request(overrides: Record<string, string> = {}, method = "POST") {
  return new NextRequest(`${publicOrigin}/api/reconciliation/runs`, {
    method,
    headers: {
      host: "127.0.0.1:3000",
      origin: publicOrigin,
      "content-type": "application/json",
      "content-length": "2",
      "x-csrf-token": session.csrfToken,
      "idempotency-key": validKey,
      ...overrides,
    },
    body: method === "POST" ? "{}" : undefined,
  });
}

const completedRun = {
  run_id: "55555555-5555-4555-8555-555555555555",
  status: "matched",
  correlation_id: "66666666-6666-4666-8666-666666666666",
  scope: "All authorized INR accounts",
  ledger_watermark: "8",
  application_version: "test",
  schema_version: "000008",
  checked_account_count: "2",
  posting_count: "2",
  mismatch_count: "0",
  started_at: "2026-08-19T11:59:58Z",
  completed_at: "2026-08-19T12:00:00Z",
};

test.beforeEach(() => {
  process.env.LEDGERSYNC_PUBLIC_ORIGIN = publicOrigin;
  process.env.LEDGERSYNC_DEPLOYMENT_ENV = "development";
  process.env.LEDGERSYNC_WEB_SESSION_SECRET = "reconciliation-command-session-secret";
});

test("reconciliation mutation requires signed write scope, exact host/origin, CSRF and POST", async () => {
  const verified = readSession(createSession(session));
  assert.ok(verified);
  assert.equal(isReconciliationMutationDenial(await authorizeReconciliationMutation(request(), verified, allow)), false);

  const denied = [
    await authorizeReconciliationMutation(request(), null, allow),
    await authorizeReconciliationMutation(request(), { ...session, scopes: ["reconciliation:read"] }, allow),
    await authorizeReconciliationMutation(request({ "x-csrf-token": "wrong" }), session, allow),
    await authorizeReconciliationMutation(request({ origin: "http://attacker.example:3000" }), session, allow),
    await authorizeReconciliationMutation(request({ host: "attacker.example:3000" }), session, allow),
  ];
  assert.deepEqual(denied.map((item) => isReconciliationMutationDenial(item) ? item.status : 0), [401, 403, 403, 403, 403]);

  const wrongMethod = await authorizeReconciliationMutation(request({}, "DELETE"), session, allow);
  assert.equal(isReconciliationMutationDenial(wrongMethod) ? wrongMethod.status : 0, 405);
  assert.equal(isReconciliationMutationDenial(wrongMethod) ? wrongMethod.headers.get("Allow") : null, "POST");
});

test("reconciliation mutation validates media type, idempotency and tenant-operator rate limit", async () => {
  assert.equal(isValidReconciliationIdempotencyKey("x".repeat(15)), false);
  assert.equal(isValidReconciliationIdempotencyKey("x".repeat(16)), true);
  assert.equal(isValidReconciliationIdempotencyKey("x".repeat(255)), true);
  assert.equal(isValidReconciliationIdempotencyKey("x".repeat(256)), false);
  assert.equal(isValidReconciliationIdempotencyKey("bad key with space"), false);

  const invalidKey = await authorizeReconciliationMutation(request({ "idempotency-key": "short" }), session, allow);
  assert.equal(isReconciliationMutationDenial(invalidKey) ? invalidKey.status : 0, 400);
  const media = await authorizeReconciliationMutation(request({ "content-type": "text/plain" }), session, allow);
  assert.equal(isReconciliationMutationDenial(media) ? media.status : 0, 415);

  let consumedKey = "";
  const deny: RateLimitStore = { consume: async (key) => { consumedKey = key; return { allowed: false, retryAfterSeconds: 27 }; } };
  const limited = await authorizeReconciliationMutation(request(), session, deny);
  assert.equal(consumedKey, "reconciliation:tenant-a:operator-a");
  assert.equal(isReconciliationMutationDenial(limited) ? limited.status : 0, 429);
  assert.equal(isReconciliationMutationDenial(limited) ? limited.headers.get("Retry-After") : null, "27");
});

test("reconciliation request body is tiny and has no caller-controlled fields", async () => {
  assert.deepEqual(parseReconciliationRunRequest({}), {});
  assert.throws(() => parseReconciliationRunRequest({ tenant_id: "attacker" }), /empty object/);
  assert.throws(() => parseReconciliationRunRequest([]), /empty object/);
  const oversized = new NextRequest(`${publicOrigin}/api/reconciliation/runs`, {
    method: "POST",
    headers: { "content-length": String(reconciliationMutationMaximumBytes + 1) },
    body: "{}",
  });
  await assert.rejects(() => readBoundedJSON(oversized, reconciliationMutationMaximumBytes), /outside the permitted size/);
});

test("reconciliation upstream success and typed failures are strict public envelopes", () => {
  assert.deepEqual(sanitizeReconciliationUpstream(202, { ...completedRun, status: "running", ledger_watermark: "", application_version: "", schema_version: "", completed_at: "", private_token: "secret" }), {
    status: 202,
    body: { ...completedRun, status: "running", ledger_watermark: "", application_version: "", schema_version: "", completed_at: "" },
  });
  assert.deepEqual(sanitizeReconciliationUpstream(409, { error: { code: "reconciliation_already_running", message: "database detail" }, run_id: completedRun.run_id, private: "secret" }), {
    status: 409,
    body: { error: { code: "reconciliation_already_running" }, run_id: completedRun.run_id },
  });
  assert.deepEqual(sanitizeReconciliationUpstream(409, { error: { code: "request_in_progress", message: "raw lock detail" }, run_id: completedRun.run_id }), {
    status: 409,
    body: { error: { code: "request_in_progress" }, run_id: completedRun.run_id },
  });
  assert.deepEqual(sanitizeReconciliationUpstream(503, { error: { code: "database_password", message: "raw" } }), {
    status: 503,
    body: { error: { code: "temporary_unavailable" } },
  });
  assert.deepEqual(sanitizeReconciliationUpstream(201, completedRun), { status: 201, body: completedRun });
  assert.deepEqual(sanitizeReconciliationUpstream(504, { error: { code: "response_unknown", message: "private commit detail" } }), {
    status: 504,
    body: { error: { code: "reconciliation_outcome_unknown" } },
  });
});

test("malformed or oversized successful reconciliation evidence is an unknown outcome", () => {
  const unknown = { status: 504, body: { error: { code: "reconciliation_outcome_unknown" } } };
  assert.deepEqual(sanitizeReconciliationUpstreamBody(202, "{truncated"), unknown);
  assert.deepEqual(sanitizeReconciliationUpstreamBody(200, "x".repeat(65_537)), unknown);
  assert.deepEqual(sanitizeReconciliationUpstream(202, { run_id: completedRun.run_id, status: "running" }), unknown);
});

test("reconciliation private and browser headers are allowlisted and timeout stays retry-safe", () => {
  assert.deepEqual(reconciliationPrivateHeaders({
    Authorization: "Bearer workload",
    "X-LedgerSync-Actor-Assertion": "actor",
    "X-Request-ID": "server-correlation",
    Cookie: "do-not-forward",
    "X-Private": "do-not-forward",
  }, validKey), {
    Authorization: "Bearer workload",
    "X-LedgerSync-Actor-Assertion": "actor",
    "X-Request-ID": "server-correlation",
    "Content-Type": "application/json",
    "Idempotency-Key": validKey,
  });
  assert.deepEqual(reconciliationResponseHeaders(new Headers({
    "idempotent-replay": "true",
    "x-request-id": "request-ref-1",
    "retry-after": "8",
    "x-private": "secret",
  })), {
    "Cache-Control": "no-store",
    "Idempotent-Replay": "true",
    "X-Request-ID": "request-ref-1",
    "Retry-After": "8",
  });
  assert.deepEqual(reconciliationDispatchError(new DOMException("timed out", "TimeoutError")), { code: "reconciliation_outcome_unknown", status: 504 });
  assert.deepEqual(reconciliationDispatchError(new Error("refused")), { code: "temporary_unavailable", status: 503 });
});

test("reconciliation POST is additive and existing GET query contract remains", async () => {
  const route = await readFile(new URL("../../src/app/api/reconciliation/runs/route.ts", import.meta.url), "utf8");
  assert.match(route, /export async function GET/);
  assert.match(route, /proxyPrivateGET\(request, session, "\/api\/reconciliation\/runs", \["cursor", "limit"\]\)/);
  assert.match(route, /export async function POST/);
  assert.match(route, /proxyReconciliationMutation\(session, authorization\.idempotencyKey\)/);
});
