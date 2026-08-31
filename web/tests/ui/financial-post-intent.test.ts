import assert from "node:assert/strict";
import test from "node:test";

import {
  createFinancialPostIntent,
  financialPostStorageKey,
  parseFinancialPostIntent,
} from "../../src/features/console/financialPostIntent";

const tenantId = "tenant-1";
const recordId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";

test("financial post recovery is scoped to domain, tenant, and immutable record", () => {
  const intent = createFinancialPostIntent(
    "funding",
    tenantId,
    recordId,
    "funding-post-1234567890",
  );
  assert.deepEqual(
    parseFinancialPostIntent(JSON.stringify(intent), "funding", tenantId, recordId),
    intent,
  );
  assert.equal(
    parseFinancialPostIntent(JSON.stringify(intent), "correction", tenantId, recordId),
    null,
  );
  assert.equal(
    parseFinancialPostIntent(JSON.stringify(intent), "funding", "tenant-2", recordId),
    null,
  );
  assert.equal(
    parseFinancialPostIntent(JSON.stringify(intent), "funding", tenantId, "other-record"),
    null,
  );
  assert.equal(
    financialPostStorageKey("funding", tenantId, recordId),
    "ledgersync:financial-post:v1:funding:tenant-1:aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
  );
});

test("financial post recovery rejects malformed, unbounded, and ambiguous values", () => {
  const intent = createFinancialPostIntent(
    "correction",
    tenantId,
    recordId,
    "correction-post-1234567890",
  );
  assert.equal(parseFinancialPostIntent("{broken", "correction", tenantId, recordId), null);
  assert.equal(parseFinancialPostIntent("x".repeat(2_049), "correction", tenantId, recordId), null);
  assert.equal(
    parseFinancialPostIntent(
      JSON.stringify({ ...intent, idempotencyKey: "short" }),
      "correction",
      tenantId,
      recordId,
    ),
    null,
  );
  assert.equal(
    parseFinancialPostIntent(
      JSON.stringify({ ...intent, unexpected: true }),
      "correction",
      tenantId,
      recordId,
    ),
    null,
  );
});
