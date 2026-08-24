import assert from "node:assert/strict";
import test from "node:test";

import { createDemoSession, readDemoConfiguration } from "../../src/lib/demo";

const localDemoEnvironment = {
  LEDGERSYNC_DEMO_MODE: "true",
  LEDGERSYNC_DEPLOYMENT_ENV: "development",
  LEDGERSYNC_PUBLIC_ORIGIN: "http://127.0.0.1:3000",
};

test("demo mode requires an affirmative development environment", () => {
  for (const deployment of [undefined, "production", "prod", "staging", "preview", "developement"]) {
    assert.throws(() => readDemoConfiguration({ ...localDemoEnvironment, LEDGERSYNC_DEPLOYMENT_ENV: deployment }), /explicit development/);
  }
  assert.throws(() => readDemoConfiguration({ ...localDemoEnvironment, LEDGERSYNC_DEPLOYMENT_ENV: undefined, NODE_ENV: "production" }), /explicit development/);
});

test("production rejects demo identity configuration even when the mode flag is absent", () => {
  assert.throws(() => readDemoConfiguration({ LEDGERSYNC_DEPLOYMENT_ENV:"preview", LEDGERSYNC_DEMO_SUBJECT_ID:"demo", LEDGERSYNC_PUBLIC_ORIGIN: "http://127.0.0.1:3000" }), /explicit development/);
});

test("demo mode requires a fixed loopback public origin", () => {
  assert.throws(() => readDemoConfiguration({ ...localDemoEnvironment, LEDGERSYNC_PUBLIC_ORIGIN: undefined }), /PUBLIC_ORIGIN is required/);
  assert.throws(() => readDemoConfiguration({ ...localDemoEnvironment, LEDGERSYNC_PUBLIC_ORIGIN: "https://ledger.example" }), /loopback public origin/);
  assert.throws(() => readDemoConfiguration({ ...localDemoEnvironment, LEDGERSYNC_PUBLIC_ORIGIN: "http://127.0.0.1:3000/path" }), /valid HTTP\(S\) origin/);
});

test("demo mode creates a narrow, expiring operator session in development", () => {
  const configuration = readDemoConfiguration({ ...localDemoEnvironment, NODE_ENV: "production" });
  const session = createDemoSession(configuration, 1_000);
  assert.equal(session.subjectId, "demo-operator");
  assert.equal(session.tenantId, "00000000-0000-4000-8000-000000000001");
  assert.equal(session.expiresAt, 1_801_000);
  assert.ok(session.scopes?.includes("transfers:write"));
  assert.ok(session.scopes?.includes("accounts:write"));
});

test("demo mode is off unless explicitly enabled", () => {
  assert.equal(readDemoConfiguration({ LEDGERSYNC_DEPLOYMENT_ENV: "development" }).enabled, false);
});
