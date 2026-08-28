import assert from "node:assert/strict";
import test from "node:test";

import {
  createAccountStorageKey,
  lifecycleAccountStorageKey,
  hasPositiveMinorUnits,
  normalizeCreateAccountFields,
  parseCreateAccountIntent,
  parseLifecycleAccountIntent,
  validAccountDisplayName,
  validAccountExternalReference,
  validLifecycleReason,
} from "../../src/features/accounts/accountCommandIntent";

const tenant = "tenant-a";
const account = "account-a";
const idempotencyKey = "account-12345678-1234-4234-8234-123456789012";

test("account command storage is tenant and account scoped", () => {
  assert.equal(createAccountStorageKey(tenant), "ledgersync:account-create:v1:tenant-a");
  assert.equal(lifecycleAccountStorageKey(tenant, account), "ledgersync:account-lifecycle:v1:tenant-a:account-a");
});

test("create drafts restore only within the original tenant and bounded schema", () => {
  const draft = { version: 1, kind: "create", tenantId: tenant, idempotencyKey, stage: "identity", request: { display_name: "", external_reference: "OPS", category: "operating", currency: "INR" } };
  assert.deepEqual(parseCreateAccountIntent(JSON.stringify(draft), tenant), draft);
  assert.equal(parseCreateAccountIntent(JSON.stringify(draft), "tenant-b"), null);
  assert.equal(parseCreateAccountIntent(JSON.stringify({ ...draft, request: { ...draft.request, currency: "USD" } }), tenant), null);
  assert.equal(parseCreateAccountIntent(JSON.stringify({ ...draft, stage: "unknown", request: { ...draft.request, display_name: "" } }), tenant), null);
});

test("client validators match account command boundaries", () => {
  assert.equal(validAccountDisplayName("Operating reserve"), true);
  assert.equal(validAccountDisplayName("bad\nname"), false);
  assert.equal(validAccountDisplayName("x".repeat(121)), false);
  assert.equal(validAccountExternalReference("OPS_INR-01"), true);
  assert.equal(validAccountExternalReference("../unsafe"), false);
  assert.equal(validLifecycleReason("Approved control review"), true);
  assert.equal(validLifecycleReason("bad\nreason"), false);
  assert.equal(validLifecycleReason(" "), false);
  assert.equal(hasPositiveMinorUnits("1"), true);
  assert.equal(hasPositiveMinorUnits("9223372036854775807"), true);
  assert.equal(hasPositiveMinorUnits("0"), false);
  assert.equal(hasPositiveMinorUnits("01"), false);
  assert.equal(hasPositiveMinorUnits("1.00"), false);
});

test("account review uses the server canonical identity before submission", () => {
  assert.deepEqual(normalizeCreateAccountFields({
    display_name: "  Operating reserve  ",
    external_reference: " OPS_INR-01 ",
    category: "operating",
    currency: "INR",
  }), {
    display_name: "Operating reserve",
    external_reference: "ops_inr-01",
    category: "operating",
    currency: "INR",
  });
});

test("unknown lifecycle retry retains exact version, reason, target and key", () => {
  const intent = { version: 1, kind: "lifecycle", tenantId: tenant, accountId: account, idempotencyKey, state: "unknown", request: { expected_version: "42", target_status: "closed", reason: "Entity retired after zero-balance review" } };
  assert.deepEqual(parseLifecycleAccountIntent(JSON.stringify(intent), tenant, account), intent);
  assert.equal(parseLifecycleAccountIntent(JSON.stringify(intent), tenant, "other-account"), null);
  assert.equal(parseLifecycleAccountIntent(JSON.stringify({ ...intent, request: { ...intent.request, expected_version: "01" } }), tenant, account), null);
  assert.equal(parseLifecycleAccountIntent(JSON.stringify({ ...intent, request: { ...intent.request, reason: "bad\nreason" } }), tenant, account), null);
});
