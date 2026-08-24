import assert from "node:assert/strict";
import test from "node:test";

import { resolveOperatorAuthorization } from "../../src/lib/oidc";

test("operator tenant, roles, and scopes come from server-owned pilot mapping", () => {
  process.env.LEDGERSYNC_OIDC_SUBJECT_PERMISSIONS = JSON.stringify({
    "operator-subject": {
      tenantId: "00000000-0000-4000-8000-000000000001",
      roles: ["tenant:operator", "platform:root"],
      scopes: ["accounts:read", "accounts:write", "reconciliation:read", "unknown:scope"],
    },
  });

  assert.deepEqual(resolveOperatorAuthorization("operator-subject"), {
    tenantId: "00000000-0000-4000-8000-000000000001",
    roles: ["tenant:operator"],
    scopes: ["accounts:read", "accounts:write", "reconciliation:read"],
  });
  assert.throws(() => resolveOperatorAuthorization("uninvited-subject"), /not invited/);
});

test("operator mapping rejects malformed or empty authorization", () => {
  for (const value of ["{}", "[]", JSON.stringify({ subject: { tenantId: "", roles: [], scopes: [] } })]) {
    process.env.LEDGERSYNC_OIDC_SUBJECT_PERMISSIONS = value;
    assert.throws(() => resolveOperatorAuthorization("subject"));
  }
});
