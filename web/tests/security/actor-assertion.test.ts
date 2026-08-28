import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

import { createActorAssertion } from "../../src/lib/actor-assertion";

test("actor assertion uses Unix seconds and bounded identity claims", () => {
  process.env.LEDGERSYNC_BFF_ASSERTION_SECRET = "actor-assertion-test-secret-long-enough";
  process.env.LEDGERSYNC_BFF_ASSERTION_ISSUER = "ledgersync-bff";
  process.env.LEDGERSYNC_BFF_ASSERTION_AUDIENCE = "ledgersync-private-api";
  process.env.LEDGERSYNC_BFF_ASSERTION_KEY_ID = "current";
  const now = new Date();
  const assertion = createActorAssertion({
    subjectId: "operator-1",
    tenantId: "tenant-a",
    csrfToken: "csrf",
    expiresAt: now.getTime() + 30 * 60_000,
    authenticatedAt: now.getTime() - 30_000,
    roles: ["tenant:operator"],
    scopes: ["accounts:read"],
  }, { now, assertionId: "assertion-contract-001" });
  const [payloadPart] = assertion.split(".");
  const payload = JSON.parse(Buffer.from(payloadPart, "base64url").toString("utf8")) as Record<string, unknown>;

  assert.equal(payload.iss, "ledgersync-bff");
  assert.equal(payload.aud, "ledgersync-private-api");
  assert.equal(payload.kid, "current");
  assert.equal(payload.jti, "assertion-contract-001");
  assert.equal(payload.iat, Math.floor(now.getTime() / 1000));
  assert.equal(payload.exp, Math.floor(now.getTime() / 1000) + 60);
  assert.equal(payload.authenticated_at, Math.floor(now.getTime() / 1000) - 30);
  assert.ok((payload.exp as number) < 10_000_000_000, "NumericDate must be Unix seconds, not milliseconds");

  const repositoryRoot = fileURLToPath(new URL("../../../", import.meta.url));
  const verification = spawnSync("go", ["run", "./cmd/assertion-contract"], {
    cwd: repositoryRoot,
    input: assertion,
    encoding: "utf8",
    env: { ...process.env, LEDGERSYNC_BFF_ASSERTION_SECRET: "actor-assertion-test-secret-long-enough" },
  });
  assert.equal(verification.status, 0, verification.stderr);
  assert.deepEqual(JSON.parse(verification.stdout), { subject_id: "operator-1", tenant_id: "tenant-a", accounts_read: true });
});
