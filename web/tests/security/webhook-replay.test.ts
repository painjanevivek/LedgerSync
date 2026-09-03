import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { NextRequest } from "next/server";

import { parseWebhookReplayApprovalRequest, parseWebhookReplayRequest, sanitizeWebhookReplayUpstreamBody } from "../../src/lib/api/webhook-replay";
import type { RateLimitStore } from "../../src/lib/rate-limit";
import type { Session } from "../../src/lib/session";
import { authorizeWebhookReplay, isWebhookReplayDenial, webhookReplayDispatchError, webhookReplayPrivateHeaders, webhookReplayResponseHeaders } from "../../src/lib/webhook-replay-boundary";

const origin = "http://127.0.0.1:3000";
const endpointId = "70000000-0000-4000-8000-000000000001";
const attemptId = "70000000-0000-4000-8000-000000000002";
const approvalId = "70000000-0000-4000-8000-000000000003";
const idempotencyKey = "webhook-replay-command-0001";
const session: Session = { subjectId: "operator-a", tenantId: "tenant-a", csrfToken: "csrf-webhook-replay", expiresAt: Date.now() + 60_000, authenticatedAt: Date.now(), scopes: ["webhooks:read", "webhooks:replay"] };
const allow: RateLimitStore = { consume: async () => ({ allowed: true, retryAfterSeconds: 0 }) };

function request(overrides: Record<string, string> = {}, method = "POST") {
  return new NextRequest(`${origin}/api/webhook-endpoints/${endpointId}/deliveries/${attemptId}/replay`, { method, headers: { host: "127.0.0.1:3000", origin, "content-type": "application/json", "x-csrf-token": session.csrfToken, "idempotency-key": idempotencyKey, ...overrides }, body: method === "POST" ? JSON.stringify({ approval_id: approvalId }) : undefined });
}

test.beforeEach(() => { process.env.LEDGERSYNC_PUBLIC_ORIGIN = origin; process.env.LEDGERSYNC_DEPLOYMENT_ENV = "development"; });

test("webhook replay requires scope, same-origin CSRF, recent authentication, UUID targets, and retry identity", async () => {
  assert.equal(isWebhookReplayDenial(await authorizeWebhookReplay(request(), session, endpointId, attemptId, "execution", allow)), false);
  const denied = [
    await authorizeWebhookReplay(request(), null, endpointId, attemptId, "execution", allow),
    await authorizeWebhookReplay(request(), { ...session, scopes: ["webhooks:read"] }, endpointId, attemptId, "execution", allow),
    await authorizeWebhookReplay(request({ "x-csrf-token": "wrong" }), session, endpointId, attemptId, "execution", allow),
    await authorizeWebhookReplay(request(), { ...session, authenticatedAt: Date.now() - 11 * 60_000 }, endpointId, attemptId, "execution", allow),
    await authorizeWebhookReplay(request(), session, "not-an-endpoint", attemptId, "execution", allow),
    await authorizeWebhookReplay(request({ "idempotency-key": "short" }), session, endpointId, attemptId, "execution", allow),
  ];
  assert.deepEqual(denied.map((value) => isWebhookReplayDenial(value) ? value.status : 0), [401, 403, 403, 403, 400, 400]);
});

test("approval and execution inputs are exact and cannot replace payload or destination", () => {
  assert.deepEqual(parseWebhookReplayApprovalRequest({ reason_code: "endpoint_restored" }), { reason_code: "endpoint_restored" });
  assert.deepEqual(parseWebhookReplayRequest({ approval_id: approvalId }), { approval_id: approvalId });
  assert.throws(() => parseWebhookReplayApprovalRequest({ reason_code: "arbitrary_reason" }), /invalid/);
  assert.throws(() => parseWebhookReplayApprovalRequest({ reason_code: "endpoint_restored", payload: { amount: "100" } }), /invalid/);
  assert.throws(() => parseWebhookReplayRequest({ approval_id: approvalId, endpoint_url: "https://attacker.example" }), /invalid/);
});

test("webhook replay exposes strict success envelopes and unknown successful ambiguity", () => {
  assert.deepEqual(sanitizeWebhookReplayUpstreamBody("approval", 201, JSON.stringify({ approval_id: approvalId, status: "approved" })), { status: 201, body: { approval_id: approvalId, status: "approved" } });
  assert.deepEqual(sanitizeWebhookReplayUpstreamBody("execution", 202, JSON.stringify({ delivery_job_id: "70000000-0000-4000-8000-000000000004", status: "scheduled" })), { status: 202, body: { delivery_job_id: "70000000-0000-4000-8000-000000000004", status: "scheduled" } });
  assert.deepEqual(sanitizeWebhookReplayUpstreamBody("execution", 202, JSON.stringify({ status: "scheduled", secret: "raw-payload" })), { status: 504, body: { error: { code: "execution_outcome_unknown" } } });
  assert.deepEqual(sanitizeWebhookReplayUpstreamBody("approval", 201, "{broken"), { status: 504, body: { error: { code: "approval_outcome_unknown" } } });
});

test("private and browser replay headers are explicit allowlists", () => {
  assert.deepEqual(webhookReplayPrivateHeaders({ Authorization: "Bearer workload", "X-LedgerSync-Actor-Assertion": "actor", "X-Request-ID": "request-1", Cookie: "never" }, idempotencyKey), { Authorization: "Bearer workload", "X-LedgerSync-Actor-Assertion": "actor", "X-Request-ID": "request-1", "Content-Type": "application/json", "Idempotency-Key": idempotencyKey });
  assert.deepEqual(webhookReplayResponseHeaders(new Headers({ "idempotent-replay": "true", "x-request-id": "request-1", "retry-after": "8", "x-secret": "never" })), { "Cache-Control": "no-store", "Idempotent-Replay": "true", "X-Request-ID": "request-1", "Retry-After": "8" });
  assert.deepEqual(webhookReplayDispatchError("execution", new DOMException("timeout", "TimeoutError")), { code: "execution_outcome_unknown", status: 504 });
});

test("browser replay routes remain fixed to immutable developer delivery commands", async () => {
  const approvalRoute = await readFile(new URL("../../src/app/api/webhook-endpoints/[endpointId]/deliveries/[attemptId]/replay-approvals/route.ts", import.meta.url), "utf8");
  const replayRoute = await readFile(new URL("../../src/app/api/webhook-endpoints/[endpointId]/deliveries/[attemptId]/replay/route.ts", import.meta.url), "utf8");
  const mutation = await readFile(new URL("../../src/lib/webhook-replay-mutation.ts", import.meta.url), "utf8");
  assert.match(approvalRoute, /parseWebhookReplayApprovalRequest/);
  assert.match(replayRoute, /parseWebhookReplayRequest/);
  assert.match(mutation, /\/api\/developer\/webhooks\/\$\{encodeURIComponent\(endpointId\)\}\/deliveries\/\$\{encodeURIComponent\(attemptId\)\}\/\$\{suffix\}/);
  assert.doesNotMatch(mutation, /endpoint_url|payload/);
});
