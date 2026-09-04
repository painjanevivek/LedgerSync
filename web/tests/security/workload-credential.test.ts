import assert from "node:assert/strict";
import test from "node:test";

import { getPrivateAPIWorkloadCredential } from "../../src/lib/workload-credential";

test("production refuses a static private API token", async () => {
  const previousEnvironment = process.env.LEDGERSYNC_ENV;
  const previousToken = process.env.LEDGERSYNC_PRIVATE_API_TOKEN;
  const previousFile = process.env.LEDGERSYNC_PRIVATE_API_TOKEN_FILE;
  Object.assign(process.env, { LEDGERSYNC_ENV: "production", LEDGERSYNC_PRIVATE_API_TOKEN: "development-static-token", LEDGERSYNC_PRIVATE_API_TOKEN_FILE: "" });
  try {
    await assert.rejects(getPrivateAPIWorkloadCredential(), /managed renewal/);
  } finally {
    if (previousEnvironment === undefined) delete process.env.LEDGERSYNC_ENV; else process.env.LEDGERSYNC_ENV = previousEnvironment;
    if (previousToken === undefined) delete process.env.LEDGERSYNC_PRIVATE_API_TOKEN; else process.env.LEDGERSYNC_PRIVATE_API_TOKEN = previousToken;
    if (previousFile === undefined) delete process.env.LEDGERSYNC_PRIVATE_API_TOKEN_FILE; else process.env.LEDGERSYNC_PRIVATE_API_TOKEN_FILE = previousFile;
  }
});

test("production exchanges OAuth client credentials for a short-lived workload token", async () => {
  const keys = [
    "LEDGERSYNC_ENV",
    "LEDGERSYNC_PRIVATE_API_TOKEN",
    "LEDGERSYNC_PRIVATE_API_TOKEN_FILE",
    "LEDGERSYNC_PRIVATE_API_TOKEN_URL",
    "LEDGERSYNC_PRIVATE_API_CLIENT_ID",
    "LEDGERSYNC_PRIVATE_API_CLIENT_SECRET",
    "LEDGERSYNC_PRIVATE_API_AUDIENCE",
  ] as const;
  const previous = new Map(keys.map((key) => [key, process.env[key]]));
  const previousFetch = globalThis.fetch;
  const payload = Buffer.from(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 300 })).toString("base64url");
  const token = `eyJhbGciOiJub25lIn0.${payload}.signature`;
  let submittedBody = "";
  Object.assign(process.env, {
    LEDGERSYNC_ENV: "production",
    LEDGERSYNC_PRIVATE_API_TOKEN: "",
    LEDGERSYNC_PRIVATE_API_TOKEN_FILE: "",
    LEDGERSYNC_PRIVATE_API_TOKEN_URL: "https://identity.example/oauth/token",
    LEDGERSYNC_PRIVATE_API_CLIENT_ID: "ledgersync-web",
    LEDGERSYNC_PRIVATE_API_CLIENT_SECRET: "test-client-secret",
    LEDGERSYNC_PRIVATE_API_AUDIENCE: "ledgersync-private-api",
  });
  globalThis.fetch = async (_input, init) => {
    submittedBody = String(init?.body);
    return new Response(JSON.stringify({ access_token: token, token_type: "Bearer", expires_in: 300 }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  };
  try {
    assert.equal(await getPrivateAPIWorkloadCredential(), token);
    const form = new URLSearchParams(submittedBody);
    assert.equal(form.get("grant_type"), "client_credentials");
    assert.equal(form.get("client_id"), "ledgersync-web");
    assert.equal(form.get("client_secret"), "test-client-secret");
    assert.equal(form.get("audience"), "ledgersync-private-api");
  } finally {
    globalThis.fetch = previousFetch;
    for (const key of keys) {
      const value = previous.get(key);
      if (value === undefined) delete process.env[key]; else process.env[key] = value;
    }
  }
});
