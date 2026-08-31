import assert from "node:assert/strict";
import test from "node:test";

import { parseWebhookReplayIntent, webhookReplayStorageKey } from "../../src/features/operations/webhookReplayIntent";

const tenantId = "tenant-a";
const endpointId = "70000000-0000-4000-8000-000000000001";
const attemptId = "70000000-0000-4000-8000-000000000002";
const approvalId = "70000000-0000-4000-8000-000000000003";
const valid = { version: 1, tenantId, endpointId, attemptId, reasonCode: "endpoint_restored", approvalKey: "webhook-approval-command-0001", executionKey: "webhook-execution-command-0001", approvalId, state: "execution_unknown" } as const;

test("webhook replay recovery storage is tenant, endpoint, and attempt scoped", () => {
  assert.notEqual(webhookReplayStorageKey(tenantId, endpointId, attemptId), webhookReplayStorageKey("tenant-b", endpointId, attemptId));
  assert.notEqual(webhookReplayStorageKey(tenantId, endpointId, attemptId), webhookReplayStorageKey(tenantId, endpointId, "70000000-0000-4000-8000-000000000009"));
  assert.deepEqual(parseWebhookReplayIntent(JSON.stringify(valid), tenantId, endpointId, attemptId), valid);
  assert.equal(parseWebhookReplayIntent(JSON.stringify(valid), "tenant-b", endpointId, attemptId), null);
});

test("unknown replay execution retains exact approval and both request keys", () => {
  assert.deepEqual(parseWebhookReplayIntent(JSON.stringify(valid), tenantId, endpointId, attemptId), valid);
  assert.equal(parseWebhookReplayIntent(JSON.stringify({ ...valid, approvalId: undefined }), tenantId, endpointId, attemptId), null);
  assert.equal(parseWebhookReplayIntent(JSON.stringify({ ...valid, executionKey: "short" }), tenantId, endpointId, attemptId), null);
  assert.equal(parseWebhookReplayIntent(JSON.stringify({ ...valid, reasonCode: "payload_changed" }), tenantId, endpointId, attemptId), null);
});

test("scheduled replay intent requires a stable delivery job reference", () => {
  assert.equal(parseWebhookReplayIntent(JSON.stringify({ ...valid, state: "scheduled" }), tenantId, endpointId, attemptId), null);
  const scheduled = { ...valid, state: "scheduled", deliveryJobId: "70000000-0000-4000-8000-000000000004" } as const;
  assert.deepEqual(parseWebhookReplayIntent(JSON.stringify(scheduled), tenantId, endpointId, attemptId), scheduled);
  assert.equal(parseWebhookReplayIntent("{broken", tenantId, endpointId, attemptId), null);
});
