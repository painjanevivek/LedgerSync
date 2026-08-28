import assert from "node:assert/strict";
import test from "node:test";

import { createLocalSession, readLocalAccessConfiguration } from "../../src/lib/local-access";

const localAccessEnvironment = {
  LEDGERSYNC_LOCAL_LOGIN_ENABLED: "true",
  LEDGERSYNC_DEPLOYMENT_ENV: "development",
  LEDGERSYNC_PUBLIC_ORIGIN: "http://127.0.0.1:3000",
};

test("local login requires an affirmative development environment", () => {
  for (const deployment of [undefined, "production", "prod", "staging", "preview", "developement"]) {
    assert.throws(() => readLocalAccessConfiguration({ ...localAccessEnvironment, LEDGERSYNC_DEPLOYMENT_ENV: deployment }), /explicit development/);
  }
  assert.throws(() => readLocalAccessConfiguration({ ...localAccessEnvironment, LEDGERSYNC_DEPLOYMENT_ENV: undefined, NODE_ENV: "production" }), /explicit development/);
});

test("production rejects local identity configuration even when the mode flag is absent", () => {
  assert.throws(() => readLocalAccessConfiguration({ LEDGERSYNC_DEPLOYMENT_ENV:"preview", LEDGERSYNC_LOCAL_SUBJECT_ID:"local-user", LEDGERSYNC_PUBLIC_ORIGIN: "http://127.0.0.1:3000" }), /explicit development/);
});

test("local login requires a fixed loopback public origin", () => {
  assert.throws(() => readLocalAccessConfiguration({ ...localAccessEnvironment, LEDGERSYNC_PUBLIC_ORIGIN: undefined }), /PUBLIC_ORIGIN is required/);
  assert.throws(() => readLocalAccessConfiguration({ ...localAccessEnvironment, LEDGERSYNC_PUBLIC_ORIGIN: "https://ledger.example" }), /loopback public origin/);
  assert.throws(() => readLocalAccessConfiguration({ ...localAccessEnvironment, LEDGERSYNC_PUBLIC_ORIGIN: "http://127.0.0.1:3000/path" }), /valid HTTP\(S\) origin/);
});

test("local login creates a narrow, expiring operator session in development", () => {
  const configuration = readLocalAccessConfiguration({ ...localAccessEnvironment, NODE_ENV: "production" });
  const session = createLocalSession(configuration, 1_000);
  assert.equal(session.subjectId, "local-user");
  assert.equal(session.tenantId, "00000000-0000-4000-8000-000000000001");
  assert.equal(session.expiresAt, 1_801_000);
  assert.ok(session.scopes?.includes("transfers:write"));
  assert.ok(session.scopes?.includes("accounts:write"));
});

test("local login is off unless explicitly enabled", () => {
  assert.equal(readLocalAccessConfiguration({ LEDGERSYNC_DEPLOYMENT_ENV: "development" }).enabled, false);
});
