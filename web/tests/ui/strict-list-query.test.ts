import assert from "node:assert/strict";
import test from "node:test";

import { parseApprovalBFFSearchParams, parseApprovalPageQuery } from "../../src/lib/page-query/approvals";
import { correctionBFFQueryRules, correctionsURL, parseCorrectionPageQuery } from "../../src/lib/page-query/corrections";
import { fundingBFFQueryRules, fundingURL, parseFundingPageQuery } from "../../src/lib/page-query/funding";
import { parseReconciliationPageQuery, reconciliationBFFQueryRules, reconciliationURL } from "../../src/lib/page-query/reconciliation";
import { isUTCDate, parseStrictListQuery, parseStrictListSearchParams } from "../../src/lib/strict-list-query";

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
  assert.equal(parseApprovalBFFSearchParams(new URLSearchParams("domain=funding&status=funding%3Arequested&limit=25")).ok, true);
  assert.equal(parseApprovalBFFSearchParams(new URLSearchParams("domain=funding&status=correction%3Arequested&limit=25")).ok, false);
  assert.equal(parseApprovalBFFSearchParams(new URLSearchParams("requested_after=2026-08-31&requested_before=2026-08-01")).ok, false);
  assert.equal(parseApprovalBFFSearchParams(new URLSearchParams("limit=101")).ok, false);
});

test("funding query accepts only a released status and opaque cursor", () => {
  assert.deepEqual(parseFundingPageQuery({ status: "posted", cursor: "opaque" }), { ok: true, filters: { status: "posted", cursor: "opaque" } });
  assert.equal(parseFundingPageQuery({ status: "settled" }).ok, false);
  assert.equal(parseFundingPageQuery({ status: ["posted", "rejected"] }).ok, false);
  assert.equal(parseFundingPageQuery({ accountId: "not-released" }).ok, false);
  assert.equal(fundingURL({ status: "approved", cursor: "next" }), "/funding?status=approved&cursor=next");
  assert.equal(parseStrictListSearchParams(new URLSearchParams("status=posted&limit=25"), fundingBFFQueryRules).ok, true);
  assert.equal(parseStrictListSearchParams(new URLSearchParams("status=settled&limit=25"), fundingBFFQueryRules).ok, false);
  assert.equal(parseStrictListSearchParams(new URLSearchParams("limit=0"), fundingBFFQueryRules).ok, false);
});

test("correction page and BFF queries share exact released semantics", () => {
  assert.deepEqual(parseCorrectionPageQuery({ status: "approved", cursor: "opaque" }), { ok: true, filters: { status: "approved", cursor: "opaque" } });
  assert.equal(parseCorrectionPageQuery({ status: "pending" }).ok, false);
  assert.equal(parseCorrectionPageQuery({ status: ["approved", "posted"] }).ok, false);
  assert.equal(parseCorrectionPageQuery({ requester: "not-released" }).ok, false);
  assert.equal(correctionsURL({ status: "posted", cursor: "next" }), "/corrections?status=posted&cursor=next");

  assert.equal(parseStrictListSearchParams(new URLSearchParams("status=posted&limit=25"), correctionBFFQueryRules).ok, true);
  assert.equal(parseStrictListSearchParams(new URLSearchParams("status=pending&limit=25"), correctionBFFQueryRules).ok, false);
  assert.equal(parseStrictListSearchParams(new URLSearchParams("status=posted&status=approved"), correctionBFFQueryRules).ok, false);
  assert.equal(parseStrictListSearchParams(new URLSearchParams("limit=101"), correctionBFFQueryRules).ok, false);
});

test("reconciliation exposes only a bounded opaque continuation", () => {
  assert.deepEqual(parseReconciliationPageQuery({ cursor: "opaque" }), { ok: true, filters: { cursor: "opaque" } });
  assert.equal(parseReconciliationPageQuery({ cursor: ["one", "two"] }).ok, false);
  assert.equal(parseReconciliationPageQuery({ status: "matched" }).ok, false);
  assert.equal(reconciliationURL({ cursor: "next" }), "/reconciliation?cursor=next");
  assert.equal(parseStrictListSearchParams(new URLSearchParams("cursor=opaque&limit=25"), reconciliationBFFQueryRules).ok, true);
  assert.equal(parseStrictListSearchParams(new URLSearchParams("limit=0"), reconciliationBFFQueryRules).ok, false);
});
