import assert from "node:assert/strict";
import test from "node:test";

import { createDemoSession, readDemoConfiguration } from "../../src/lib/demo";

test("demo mode is rejected in production", () => {
  assert.throws(() => readDemoConfiguration({ LEDGERSYNC_DEMO_MODE: "true", LEDGERSYNC_DEPLOYMENT_ENV: "production" }), /forbidden/);
});

test("demo mode creates a narrow, expiring operator session in development", () => {
  const configuration = readDemoConfiguration({ LEDGERSYNC_DEMO_MODE: "true", LEDGERSYNC_DEPLOYMENT_ENV: "development" });
  const session = createDemoSession(configuration, 1_000);
  assert.equal(session.subjectId, "demo-operator");
  assert.equal(session.tenantId, "00000000-0000-4000-8000-000000000001");
  assert.equal(session.expiresAt, 1_801_000);
  assert.ok(session.scopes?.includes("transfers:write"));
});

test("demo mode is off unless explicitly enabled", () => {
  assert.equal(readDemoConfiguration({ LEDGERSYNC_DEPLOYMENT_ENV: "development" }).enabled, false);
});
