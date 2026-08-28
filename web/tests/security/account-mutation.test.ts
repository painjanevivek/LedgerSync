import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { NextRequest } from "next/server";

import {
  accountMutationDispatchError,
  accountMutationPrivateHeaders,
  accountMutationResponseHeaders,
  authorizeAccountMutation,
  isAccountMutationDenial,
} from "../../src/lib/account-mutation-boundary";
import {
  accountMutationMaximumBytes,
  isValidAccountIdempotencyKey,
  parseCreateAccountRequest,
  parseUpdateAccountRequest,
  sanitizeAccountUpstream,
  sanitizeAccountUpstreamBody,
} from "../../src/lib/api/accounts";
import { readBoundedJSON } from "../../src/lib/security";
import { createSession, readSession, type Session } from "../../src/lib/session";

const publicOrigin = "http://127.0.0.1:3000";
const validKey = "account-command-0001";
const session: Session = {
  subjectId: "operator-a",
  tenantId: "tenant-a",
  csrfToken: "csrf-account-command",
  expiresAt: Date.now() + 60_000,
  roles: ["tenant:operator"],
  scopes: ["accounts:read", "accounts:write"],
};

function request(method: string, overrides: Record<string, string> = {}): NextRequest {
  return new NextRequest(`${publicOrigin}/api/me/accounts`, {
    method,
    headers: {
      host: "127.0.0.1:3000",
      origin: publicOrigin,
      "content-type": "application/json",
      "x-csrf-token": session.csrfToken,
      "idempotency-key": validKey,
      ...overrides,
    },
  });
}

test.beforeEach(() => {
  process.env.LEDGERSYNC_PUBLIC_ORIGIN = publicOrigin;
  process.env.LEDGERSYNC_DEPLOYMENT_ENV = "development";
  process.env.LEDGERSYNC_WEB_SESSION_SECRET = "account-command-session-test-secret";
  process.env.LEDGERSYNC_PRIVATE_API_URL = "http://private-api:8080";
  process.env.LEDGERSYNC_PRIVATE_API_TOKEN = "account-command-workload-token";
  process.env.LEDGERSYNC_BFF_ASSERTION_SECRET = "account-command-assertion-test-secret";
});

test("account mutation authorization requires a signed scoped session and exact method", async () => {
  const signed = createSession(session);
  const verified = readSession(signed);
  assert.ok(verified);
  assert.equal(isAccountMutationDenial(authorizeAccountMutation(request("POST"), verified, "POST")), false);

  const tampered = readSession(`${signed}x`);
  const noSession = authorizeAccountMutation(request("POST"), tampered, "POST");
  assert.equal(isAccountMutationDenial(noSession) ? noSession.status : undefined, 401);

  const noScope = authorizeAccountMutation(request("POST"), { ...session, scopes: ["accounts:read"] }, "POST");
  assert.equal(isAccountMutationDenial(noScope) ? noScope.status : undefined, 403);

  const wrongMethod = authorizeAccountMutation(request("DELETE"), session, "POST");
  assert.equal(isAccountMutationDenial(wrongMethod) ? wrongMethod.status : undefined, 405);
  assert.equal(isAccountMutationDenial(wrongMethod) ? wrongMethod.headers.get("Allow") : undefined, "POST");
  assert.equal(isAccountMutationDenial(wrongMethod) ? wrongMethod.headers.get("Cache-Control") : undefined, "no-store");

  const wrongContentType = authorizeAccountMutation(request("POST", { "content-type": "text/plain" }), session, "POST");
  assert.equal(isAccountMutationDenial(wrongContentType) ? wrongContentType.status : undefined, 415);
});

test("account mutation authorization rejects CSRF, cross-origin, and DNS-rebinding requests", () => {
  const missingToken = authorizeAccountMutation(request("POST", { "x-csrf-token": "" }), session, "POST");
  assert.equal(isAccountMutationDenial(missingToken) ? missingToken.status : undefined, 403);

  const crossOrigin = authorizeAccountMutation(request("POST", { origin: "http://attacker.example:3000" }), session, "POST");
  assert.equal(isAccountMutationDenial(crossOrigin) ? crossOrigin.status : undefined, 403);

  const reboundHost = authorizeAccountMutation(request("POST", { host: "attacker.example:3000" }), session, "POST");
  assert.equal(isAccountMutationDenial(reboundHost) ? reboundHost.status : undefined, 403);

  delete process.env.LEDGERSYNC_PUBLIC_ORIGIN;
  const missingFixedOrigin = authorizeAccountMutation(request("POST"), session, "POST");
  assert.equal(isAccountMutationDenial(missingFixedOrigin) ? missingFixedOrigin.status : undefined, 403);
});

test("idempotency keys are 16-255 visible ASCII characters without reinterpretation", () => {
  assert.equal(isValidAccountIdempotencyKey("x".repeat(15)), false);
  assert.equal(isValidAccountIdempotencyKey("x".repeat(16)), true);
  assert.equal(isValidAccountIdempotencyKey("x".repeat(255)), true);
  assert.equal(isValidAccountIdempotencyKey("x".repeat(256)), false);
  assert.equal(isValidAccountIdempotencyKey(" account-command-1"), false);
  assert.equal(isValidAccountIdempotencyKey("account-command\n1"), false);

  const invalid = authorizeAccountMutation(request("POST", { "idempotency-key": "short" }), session, "POST");
  assert.equal(isAccountMutationDenial(invalid) ? invalid.status : undefined, 400);
});

test("account JSON is limited to 4 KiB and strict command-specific allowlists", async () => {
  const create = {
    display_name: "Operating account",
    external_reference: "ops-inr",
    category: "operating",
    currency: "INR",
  };
  assert.deepEqual(parseCreateAccountRequest(create), create);
  assert.throws(() => parseCreateAccountRequest({ ...create, tenant_id: "attacker" }), /schema/);
  assert.throws(() => parseCreateAccountRequest({ ...create, currency: "USD" }), /fields/);
  assert.throws(() => parseCreateAccountRequest({ ...create, display_name: "x".repeat(121) }), /fields/);
  assert.throws(() => parseCreateAccountRequest({ ...create, external_reference: "x".repeat(65) }), /fields/);
  assert.throws(() => parseCreateAccountRequest({ ...create, external_reference: "../unsafe" }), /fields/);

  assert.deepEqual(parseUpdateAccountRequest({ expected_version: "1", target_status: "frozen", reason: "Suspected duplicate instructions" }), { expected_version: "1", target_status: "frozen", reason: "Suspected duplicate instructions" });
  assert.deepEqual(parseUpdateAccountRequest({ expected_version: "2", display_name: "Reserve", external_reference: "reserve-inr", category: "reserve" }), { expected_version: "2", display_name: "Reserve", external_reference: "reserve-inr", category: "reserve" });
  assert.throws(() => parseUpdateAccountRequest({ expected_version: 1, target_status: "frozen", reason: "test" }), /schema/);
  assert.throws(() => parseUpdateAccountRequest({ expected_version: "01", target_status: "frozen", reason: "test" }), /schema/);
  assert.throws(() => parseUpdateAccountRequest({ expected_version: "1", target_status: "frozen" }), /supported command/);
  assert.throws(() => parseUpdateAccountRequest({ expected_version: "1", target_status: "frozen", reason: "   " }), /supported command/);
  assert.throws(() => parseUpdateAccountRequest({ expected_version: "1", target_status: "frozen", reason: "bad\nreason" }), /supported command/);
  assert.throws(() => parseUpdateAccountRequest({ expected_version: "1", target_status: "frozen", reason: "x".repeat(257) }), /supported command/);
  assert.throws(() => parseUpdateAccountRequest({ expected_version: "1", target_status: "frozen", reason: "test", display_name: "mixed" }), /supported command/);
  assert.throws(() => parseUpdateAccountRequest({ expected_version: "2", display_name: "Reserve", external_reference: "reserve-inr", category: "reserve", reason: "not metadata" }), /supported command/);

  const oversized = new NextRequest(`${publicOrigin}/api/me/accounts`, {
    method: "POST",
    headers: { "content-length": String(accountMutationMaximumBytes + 1) },
    body: "{}",
  });
  await assert.rejects(() => readBoundedJSON(oversized, accountMutationMaximumBytes), /outside the permitted size/);
  const concealedOversize = new NextRequest(`${publicOrigin}/api/me/accounts`, {
    method: "POST",
    headers: { "content-length": "2" },
    body: JSON.stringify({ value: "x".repeat(accountMutationMaximumBytes) }),
  });
  await assert.rejects(() => readBoundedJSON(concealedOversize, accountMutationMaximumBytes), /outside the permitted size/);
});

test("account upstream responses expose only the public result or sanitized error envelope", () => {
  const success = sanitizeAccountUpstream(201, {
    account_id: "70000000-0000-4000-8000-000000000001",
    tenant_id: "00000000-0000-4000-8000-000000000001",
    account_version: "1",
    status: "active",
    currency: "INR",
    display_name: "Operating account",
    external_reference: "ops-inr",
    category: "operating",
    available_minor: "0",
    ledger_minor: "0",
    created_at: "2026-08-25T10:00:00Z",
    updated_at: "2026-08-25T10:00:00Z",
    credential: "private-token",
    debug: "database host",
  });
  assert.deepEqual(success, {
    status: 201,
    body: {
      account_id: "70000000-0000-4000-8000-000000000001",
      tenant_id: "00000000-0000-4000-8000-000000000001",
      currency: "INR",
      status: "active",
      display_name: "Operating account",
      external_reference: "ops-inr",
      category: "operating",
      account_version: "1",
      available_minor: "0",
      ledger_minor: "0",
      created_at: "2026-08-25T10:00:00Z",
      updated_at: "2026-08-25T10:00:00Z",
    },
  });

  const failure = sanitizeAccountUpstream(409, {
    error: { code: "account_version_conflict", message: "secret database detail", stack: "private stack" },
    credential: "private-token",
  });
  assert.deepEqual(failure, { status: 409, body: { error: { code: "account_version_conflict" } } });
  assert.deepEqual(sanitizeAccountUpstream(422, { error: { code: "account_not_zero", message: "raw balance detail" } }), {
    status: 422,
    body: { error: { code: "account_not_zero" } },
  });
  assert.deepEqual(sanitizeAccountUpstream(400, { error: { code: "invalid_request", message: "raw parser detail" } }), {
    status: 400,
    body: { error: { code: "invalid_request" } },
  });
  assert.deepEqual(sanitizeAccountUpstream(409, { error: { code: "unauthorized", message: "mismatched typed error" } }), {
    status: 503,
    body: { error: { code: "temporary_unavailable" } },
  });
  assert.deepEqual(sanitizeAccountUpstream(500, { error: { code: "secret_database_password", message: "raw" } }), {
    status: 503,
    body: { error: { code: "temporary_unavailable" } },
  });
});

test("unprovable successful account responses are explicit unknown outcomes", () => {
  const unknown = { status: 504, body: { error: { code: "account_command_outcome_unknown" } } };
  assert.deepEqual(sanitizeAccountUpstreamBody(201, "{truncated"), unknown);
  assert.deepEqual(sanitizeAccountUpstreamBody(200, "x".repeat(65_537)), unknown);
  assert.deepEqual(sanitizeAccountUpstream(201, {
    account_id: "70000000-0000-4000-8000-000000000001",
    account_version: "1",
    status: "active",
  }), unknown);
});

test("typed private API rejections and unavailable responses retain non-success semantics", () => {
  assert.deepEqual(sanitizeAccountUpstreamBody(422, JSON.stringify({ error: { code: "account_not_zero", message: "not forwarded" } })), {
    status: 422,
    body: { error: { code: "account_not_zero" } },
  });
  assert.deepEqual(sanitizeAccountUpstreamBody(503, JSON.stringify({ error: { code: "temporary_unavailable", message: "retry identical key" } })), {
    status: 503,
    body: { error: { code: "temporary_unavailable" } },
  });
  assert.deepEqual(sanitizeAccountUpstreamBody(500, "{malformed"), {
    status: 503,
    body: { error: { code: "temporary_unavailable" } },
  });
});

test("account mutation private and response headers are explicit allowlists", () => {
  const privateHeaders = accountMutationPrivateHeaders({
    Authorization: "Bearer workload",
    "X-LedgerSync-Actor-Assertion": "actor-assertion",
    "X-Request-ID": "server-request-id",
    Cookie: "must-not-forward",
    "X-Private-Credential": "must-not-forward",
  }, validKey);
  assert.deepEqual(privateHeaders, {
    Authorization: "Bearer workload",
    "X-LedgerSync-Actor-Assertion": "actor-assertion",
    "X-Request-ID": "server-request-id",
    "Content-Type": "application/json",
    "Idempotency-Key": validKey,
  });

  const publicHeaders = accountMutationResponseHeaders(new Headers({
    "idempotent-replay": "true",
    "x-request-id": "request-123",
    "retry-after": "5",
    "x-private-credential": "must-not-forward",
  }));
  assert.deepEqual(publicHeaders, {
    "Cache-Control": "no-store",
    "Idempotent-Replay": "true",
    "X-Request-ID": "request-123",
    "Retry-After": "5",
  });
  assert.deepEqual(accountMutationResponseHeaders(new Headers({
    "idempotent-replay": "secret",
    "x-request-id": "bad value",
    "retry-after": "not-a-number",
  })), { "Cache-Control": "no-store" });
});

test("account mutation dispatch timeout is an explicit unknown outcome", () => {
  assert.deepEqual(accountMutationDispatchError(new DOMException("timed out", "TimeoutError")), {
    code: "account_command_outcome_unknown",
    status: 504,
  });
  assert.deepEqual(accountMutationDispatchError(new Error("connection refused")), {
    code: "temporary_unavailable",
    status: 503,
  });
});

test("existing account GET routes retain their paths and query allowlists", async () => {
  const listRoute = await readFile(new URL("../../src/app/api/me/accounts/route.ts", import.meta.url), "utf8");
  const detailRoute = await readFile(new URL("../../src/app/api/accounts/[accountId]/route.ts", import.meta.url), "utf8");
  const privateProxy = await readFile(new URL("../../src/lib/private-api.ts", import.meta.url), "utf8");
  assert.match(listRoute, /export async function GET/);
  assert.match(listRoute, /proxyPrivateGET\(request, session, "\/api\/me\/accounts", \["cursor", "limit", "q", "status", "category"\]\)/);
  assert.match(detailRoute, /export async function GET/);
  assert.match(detailRoute, /proxyPrivateGET\(request, session, `\/api\/accounts\/\$\{encodeURIComponent\(accountId\)\}`, \[\]\)/);
  assert.match(detailRoute, /proxyAccountMutation\(session, "PATCH", `\/api\/accounts\/\$\{encodeURIComponent\(accountId\)\}`/);
  assert.match(privateProxy, /new Set\(allowedQuery\)/);
  assert.match(privateProxy, /searchParams\.getAll\(key\)\.length !== 1/);
  assert.match(privateProxy, /!permitted\.has\(key\)/);
  assert.match(privateProxy, /jsonError\("invalid_request", 400\)/);
});
