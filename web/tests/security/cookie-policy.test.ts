import assert from "node:assert/strict";
import test from "node:test";
import { authenticationCookiePolicy } from "../../src/lib/cookie-policy";

test("optimized local builds issue browser-accepted cookies without secure prefixes", () => {
  const policy = authenticationCookiePolicy({ NODE_ENV: "production", LEDGERSYNC_DEPLOYMENT_ENV: "development", LEDGERSYNC_COOKIE_SECURE: "false" });
  assert.deepEqual(policy, { secure: false, sessionName: "ledgersync_session", transactionName: "ledgersync_oidc_transaction" });
});

test("production authentication cookies always satisfy prefix requirements", () => {
  for (const environment of [
    { NODE_ENV: "production" },
    { NODE_ENV: "development", LEDGERSYNC_DEPLOYMENT_ENV: "production", LEDGERSYNC_COOKIE_SECURE: "false" },
    { LEDGERSYNC_ENV: "prod", LEDGERSYNC_COOKIE_SECURE: "false" },
  ]) {
    assert.deepEqual(authenticationCookiePolicy(environment), { secure: true, sessionName: "__Host-ledgersync_session", transactionName: "__Secure-ledgersync_oidc_transaction" });
  }
});
