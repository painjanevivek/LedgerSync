import assert from "node:assert/strict";
import test from "node:test";

import {
  createRateLimitStore,
  InMemoryRateLimitStore,
  rateLimitDenial,
  requiresSharedRateLimit,
  SharedRateLimitStore,
} from "../../src/lib/rate-limit";

test("deployment policy requires the shared limiter on production and every Vercel environment", () => {
  assert.equal(requiresSharedRateLimit({ NODE_ENV: "development" }), false);
  assert.equal(requiresSharedRateLimit({ LEDGERSYNC_DEPLOYMENT_ENV: "production" }), true);
  assert.equal(requiresSharedRateLimit({ NODE_ENV: "development", VERCEL: "1" }), true);
  assert.ok(createRateLimitStore({ NODE_ENV: "test" }) instanceof InMemoryRateLimitStore);
});

test("shared limiter uses an opaque key and enforces the atomic counter result", async () => {
  let observedKey = "";
  const store = new SharedRateLimitStore(async (key) => {
    observedKey = key;
    return [4, 19];
  }, "preview", "test-only-rate-limit-key-secret-32-bytes");
  const decision = await store.consume("tenant-secret:operator-secret", 3, 60);
  assert.deepEqual(decision, { allowed: false, retryAfterSeconds: 19 });
  assert.match(observedKey, /^ledgersync:preview:bff-rate:[0-9a-f]{64}$/);
  assert.doesNotMatch(observedKey, /tenant-secret|operator-secret/);
});

test("shared limiter fails closed as temporary unavailable instead of a false 429", async () => {
  const store = new SharedRateLimitStore(async () => { throw new Error("redis unavailable"); }, "test", "test-only-rate-limit-key-secret-32-bytes");
  const decision = await store.consume("tenant:operator", 3, 60);
  assert.deepEqual(decision, { allowed: false, retryAfterSeconds: 0, available: false });
  assert.deepEqual(rateLimitDenial(decision), { code: "temporary_unavailable", status: 503 });
  assert.deepEqual(rateLimitDenial({ allowed: false, retryAfterSeconds: 7 }), { code: "rate_limited", status: 429, retryAfterSeconds: 7 });
});
