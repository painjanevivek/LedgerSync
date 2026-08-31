import assert from "node:assert/strict";
import test from "node:test";

import { parseApprovalPageQuery } from "../../src/lib/page-query/approvals";
import { isUTCDate, parseStrictListQuery } from "../../src/lib/strict-list-query";

test("strict list queries reject unknown repeated empty oversized and malformed values", () => {
  const rules = { status: { values: ["active"], maximumLength: 16 } } as const;
  assert.equal(parseStrictListQuery({ unknown: "value" }, rules).ok, false);
  assert.equal(parseStrictListQuery({ status: ["active", "active"] }, rules).ok, false);
  assert.equal(parseStrictListQuery({ status: " " }, rules).ok, false);
  assert.equal(parseStrictListQuery({ status: "a".repeat(17) }, rules).ok, false);
  assert.equal(parseStrictListQuery({ status: "disabled" }, rules).ok, false);
  assert.deepEqual(parseStrictListQuery({ status: " active " }, rules), { ok: true, values: { status: "active" } });
});

test("UTC date validation rejects rollover dates", () => {
  assert.equal(isUTCDate("2026-08-31"), true);
  assert.equal(isUTCDate("2026-02-29"), false);
  assert.equal(isUTCDate("31-08-2026"), false);
});

test("approval page query retains an exact compatible investigation", () => {
  const parsed = parseApprovalPageQuery({
    domain: "funding",
    status: "funding:requested",
    requester: "operator-1",
    requested_after: "2026-08-01",
    requested_before: "2026-08-31",
    actionable_by_me: "true",
    cursor: "opaque-page",
  });
  assert.equal(parsed.ok, true);
  if (parsed.ok) assert.deepEqual(parsed.filters, {
    domain: "funding",
    status: "funding:requested",
    requester: "operator-1",
    age: "",
    requestedAfter: "2026-08-01",
    requestedBefore: "2026-08-31",
    actionableByMe: true,
    cursor: "opaque-page",
  });
});

test("approval page rejects contradictory domains and reversed dates", () => {
  assert.equal(parseApprovalPageQuery({ domain: "funding", status: "correction:requested" }).ok, false);
  assert.equal(parseApprovalPageQuery({ requested_after: "2026-08-31", requested_before: "2026-08-01" }).ok, false);
});
