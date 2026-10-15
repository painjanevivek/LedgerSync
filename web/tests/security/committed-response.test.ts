import assert from "node:assert/strict";
import test from "node:test";

import { applyConsistencySessionMetadata } from "@/lib/committed-response";
import type { Session } from "@/lib/session";

const session: Session = {
  subjectId: "operator-1",
  tenantId: "tenant-1",
  csrfToken: "csrf-1",
  expiresAt: Date.now() + 60_000,
  consistencyRequirements: { "account-existing": "token-existing" },
};

test("committed transfer metadata merges bounded requirements into a re-signed session", () => {
  const cookies: string[] = [];
  const status = applyConsistencySessionMetadata(
    JSON.stringify({ "account-new": "token-new" }),
    session,
    (value) => cookies.push(value),
    (payload) => JSON.stringify(payload),
  );

  assert.equal(status, "complete");
  assert.equal(cookies.length, 1);
  assert.deepEqual(JSON.parse(cookies[0]).consistencyRequirements, {
    "account-existing": "token-existing",
    "account-new": "token-new",
  });
});

for (const scenario of [
  { name: "malformed header", serialized: "not-json", setCookie: () => {}, signer: (payload: Session) => JSON.stringify(payload) },
  { name: "session signer failure", serialized: JSON.stringify({ account: "token" }), setCookie: () => {}, signer: (): string => { throw new Error("signing unavailable"); } },
  { name: "cookie attachment failure", serialized: JSON.stringify({ account: "token" }), setCookie: (): void => { throw new Error("cookie unavailable"); }, signer: (payload: Session) => JSON.stringify(payload) },
]) {
  test(`committed transfer metadata flags ${scenario.name} without throwing`, () => {
    assert.doesNotThrow(() => applyConsistencySessionMetadata(scenario.serialized, session, scenario.setCookie, scenario.signer));
    assert.equal(applyConsistencySessionMetadata(scenario.serialized, session, scenario.setCookie, scenario.signer), "unavailable");
  });
}
