import assert from "node:assert/strict";
import test from "node:test";

import {
  createStoredTransferIntent,
  parseStoredTransferIntent,
  storedIntentMatches,
  transferIntentStorageKey,
} from "../../src/features/transfers/transferIntent";

const source = { account_id: "source", currency: "INR", status: "active" as const, available_minor: "1000", ledger_minor: "1000", version: "1", as_of: "2026-08-24T00:00:00Z" };
const destination = { account_id: "destination", currency: "INR", status: "active" as const, available_minor: "0", ledger_minor: "0", version: "1", as_of: "2026-08-24T00:00:00Z" };
const prepared = { source, destination, amountMinor: "1250" };

test("a stored retry key is bound to the complete canonical transfer intent", () => {
  const intent = createStoredTransferIntent("12345678-1234-4234-8234-123456789012", prepared);
  assert.equal(storedIntentMatches(intent, prepared), true);
  assert.equal(storedIntentMatches(intent, { ...prepared, amountMinor: "1251" }), false);
  assert.equal(storedIntentMatches(intent, { ...prepared, destination: source }), false);
  assert.equal(transferIntentStorageKey("tenant-a"), "ledgersync.transfer.intent.tenant-a");
});

test("stored intent parsing fails closed for corrupt or ambiguous financial input", () => {
  const valid = createStoredTransferIntent("12345678-1234-4234-8234-123456789012", prepared);
  assert.deepEqual(parseStoredTransferIntent(JSON.stringify(valid)), valid);
  assert.equal(parseStoredTransferIntent("not-json"), null);
  assert.equal(parseStoredTransferIntent(JSON.stringify({ ...valid, amountMinor: "12.50" })), null);
  assert.equal(parseStoredTransferIntent(JSON.stringify({ ...valid, currency: "inr" })), null);
  assert.equal(parseStoredTransferIntent(JSON.stringify({ ...valid, destinationAccountId: valid.sourceAccountId })), null);
});
