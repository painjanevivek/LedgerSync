import assert from "node:assert/strict";
import test from "node:test";

import {
  parseReconciliationCommandIntent,
  reconciliationCommandStorageKey,
} from "../../src/features/reconciliation/reconciliationCommandIntent";

const tenant = "tenant-a";
const valid = {
  version: 1,
  tenantId: tenant,
  idempotencyKey: "reconcile-command-0001",
  state: "unknown",
  submittedAt: "2026-08-25T12:00:00Z",
} as const;

test("reconciliation command storage is tenant scoped", () => {
  assert.notEqual(reconciliationCommandStorageKey("tenant-a"), reconciliationCommandStorageKey("tenant-b"));
  assert.equal(parseReconciliationCommandIntent(JSON.stringify(valid), tenant)?.idempotencyKey, valid.idempotencyKey);
  assert.equal(parseReconciliationCommandIntent(JSON.stringify(valid), "tenant-b"), null);
});

test("unknown reconciliation retry retains the exact request key", () => {
  assert.deepEqual(parseReconciliationCommandIntent(JSON.stringify(valid), tenant), valid);
  assert.equal(parseReconciliationCommandIntent(JSON.stringify({ ...valid, idempotencyKey: "short" }), tenant), null);
  assert.equal(parseReconciliationCommandIntent(JSON.stringify({ ...valid, idempotencyKey: "bad key with space" }), tenant), null);
  assert.equal(parseReconciliationCommandIntent(JSON.stringify({ ...valid, state: "running" }), tenant), null);
  assert.equal(parseReconciliationCommandIntent(JSON.stringify({ ...valid, state: "running", runId: "not-a-run" }), tenant), null);
});

test("running reconciliation intent binds one stable run ID", () => {
  const running = {
    ...valid,
    state: "running",
    runId: "55555555-5555-4555-8555-555555555555",
  } as const;
  assert.deepEqual(parseReconciliationCommandIntent(JSON.stringify(running), tenant), running);
  assert.equal(parseReconciliationCommandIntent("{broken", tenant), null);
  assert.equal(parseReconciliationCommandIntent("x".repeat(2_049), tenant), null);
});

