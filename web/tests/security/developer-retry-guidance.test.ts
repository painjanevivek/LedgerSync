import test from "node:test";
import assert from "node:assert/strict";
import { operationRetryGuidance } from "../../src/features/developer/retry-guidance";

test("developer guidance follows the declared retry contract instead of assigning a key to every mutation", () => {
  assert.match(operationRetryGuidance({ operation_id: "createTransfer", method: "POST" }), /Same key \+ same intent replays/);
  assert.match(operationRetryGuidance({ operation_id: "approveFundingEvent", method: "POST" }), /does not declare a retry key/);
  assert.match(operationRetryGuidance({ operation_id: "runReconciliation", method: "POST" }), /Inspect run state/);
  assert.match(operationRetryGuidance({ operation_id: "getOwnedAccount", method: "GET" }), /freshness and authorization/);
});
