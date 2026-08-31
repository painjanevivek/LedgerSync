import assert from "node:assert/strict";
import test from "node:test";

import { accountDetailURL, accountDirectoryURL, accountFiltersFromReturnPath, parseAccountBFFSearchParams, parseAccountHistoryBFFSearchParams, parseAccountPageQuery } from "../../src/lib/page-query/accounts";
import { parseApprovalBFFSearchParams, parseApprovalPageQuery } from "../../src/lib/page-query/approvals";
import { correctionBFFQueryRules, correctionsURL, parseCorrectionPageQuery } from "../../src/lib/page-query/corrections";
import { fundingBFFQueryRules, fundingURL, parseFundingPageQuery } from "../../src/lib/page-query/funding";
import { parseReconciliationPageQuery, reconciliationBFFQueryRules, reconciliationURL } from "../../src/lib/page-query/reconciliation";
import { eventBFFQueryRules, eventsURL, parseEventBFFSearchParams, parseEventPageQuery, parseWebhookBFFSearchParams, parseWebhookPageQuery, webhookBFFQueryRules, webhooksURL } from "../../src/lib/page-query/operations";
import { parseTransferPageQuery, parseTransferSearchParams, transferBFFQueryRules, transferExportQuery, transferURL } from "../../src/lib/page-query/transfers";
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

test("transfer page, BFF, URL, and export retain one exact filter set", () => {
  const parsed = parseTransferPageQuery({
    q: "ABC-1",
    accountId: "10000000-0000-4000-8000-000000000001",
    status: "pending",
    from: "2026-08-01T00:00:00Z",
    to: "2026-08-31T23:59:59Z",
    cursor: "opaque",
  });
  assert.deepEqual(parsed, { ok: true, filters: {
    query: "abc-1",
    accountId: "10000000-0000-4000-8000-000000000001",
    status: "pending",
    from: "2026-08-01T00:00:00.000Z",
    to: "2026-08-31T23:59:59.000Z",
    cursor: "opaque",
  }, preferredDestinationId: undefined, returnTo: undefined });
  assert.equal(parseTransferPageQuery({ status: ["posted", "rejected"] }).ok, false);
  assert.equal(parseTransferPageQuery({ accountId: "not-a-uuid" }).ok, false);
  assert.equal(parseTransferPageQuery({ q: "customer@example.com" }).ok, false);
  assert.equal(parseTransferPageQuery({ from: "2026-02-30T00:00:00Z" }).ok, false);
  assert.equal(parseTransferPageQuery({ from: "2026-09-01T00:00:00Z", to: "2026-08-01T00:00:00Z" }).ok, false);
  assert.equal(parseTransferPageQuery({ loadedPageOnly: "true" }).ok, false);

  const filters = parsed.ok ? parsed.filters : { query: "", accountId: "", status: "" as const, from: "", to: "" };
  assert.equal(transferURL(filters), "/transfers?q=abc-1&accountId=10000000-0000-4000-8000-000000000001&status=pending&from=2026-08-01T00%3A00%3A00.000Z&to=2026-08-31T23%3A59%3A59.000Z&cursor=opaque");
  assert.equal(transferExportQuery(filters).toString(), "limit=10000&q=abc-1&accountId=10000000-0000-4000-8000-000000000001&status=pending&from=2026-08-01T00%3A00%3A00.000Z&to=2026-08-31T23%3A59%3A59.000Z");
  assert.equal(parseTransferSearchParams(new URLSearchParams("q=abc-1&status=pending&limit=25"), transferBFFQueryRules).ok, true);
  assert.equal(parseTransferSearchParams(new URLSearchParams("q=ops-1&limit=25"), transferBFFQueryRules).ok, false);
});

test("event and webhook pages reject malformed investigations before BFF reads", () => {
  assert.deepEqual(parseEventPageQuery({ eventType: "transfer.posted", state: "dead", from: "2026-08-01T00:00:00Z", to: "2026-08-31T23:59:59Z", cursor: "event-next" }), { ok: true, filters: {
    eventType: "transfer.posted", state: "dead", endpointId: undefined, relatedId: undefined, correlationId: undefined,
    from: "2026-08-01T00:00:00Z", to: "2026-08-31T23:59:59Z", cursor: "event-next",
  } });
  assert.equal(parseEventPageQuery({ state: ["dead", "published"] }).ok, false);
  assert.equal(parseEventPageQuery({ endpointId: "not-a-uuid" }).ok, false);
  assert.equal(parseEventPageQuery({ from: "2026-02-30T00:00:00Z" }).ok, false);
  assert.equal(parseEventPageQuery({ from: "2026-09-01T00:00:00Z", to: "2026-08-01T00:00:00Z" }).ok, false);
  assert.equal(eventsURL({ state: "dead", cursor: "event-next" }), "/events?state=dead&cursor=event-next");
  assert.equal(parseEventBFFSearchParams(new URLSearchParams("state=dead&limit=25")).ok, true);
  assert.equal(parseStrictListSearchParams(new URLSearchParams("state=dead&limit=25"), eventBFFQueryRules).ok, true);

  assert.deepEqual(parseWebhookPageQuery({ status: "active", eventType: "transfer.posted", cursor: "webhook-next" }), { ok: true, filters: { status: "active", eventType: "transfer.posted", cursor: "webhook-next" } });
  assert.equal(parseWebhookPageQuery({ status: "paused" }).ok, false);
  assert.equal(parseWebhookPageQuery({ eventType: "*" }).ok, false);
  assert.equal(webhooksURL({ status: "active", cursor: "webhook-next" }), "/webhooks?status=active&cursor=webhook-next");
  assert.equal(parseWebhookBFFSearchParams(new URLSearchParams("status=active&limit=25")).ok, true);
  assert.equal(parseStrictListSearchParams(new URLSearchParams("status=active&limit=101"), webhookBFFQueryRules).ok, false);
});

test("account page, BFF, detail return, and history queries retain one bounded contract", () => {
  const accountId = "10000000-0000-4000-8000-000000000001";
  const parsed = parseAccountPageQuery({ q: " ACME-01 ", status: "active", category: "operating", cursor: "account-next", focus: accountId });
  assert.deepEqual(parsed, {
    ok: true,
    filters: { query: "ACME-01", status: "active", category: "operating", cursor: "account-next" },
    focusAccountId: accountId,
  });
  assert.equal(parseAccountPageQuery({ status: ["active", "closed"] }).ok, false);
  assert.equal(parseAccountPageQuery({ category: "unknown" }).ok, false);
  assert.equal(parseAccountPageQuery({ q: "unsafe\nquery" }).ok, false);
  assert.equal(parseAccountPageQuery({ cursor: "x".repeat(513) }).ok, false);
  assert.equal(parseAccountPageQuery({ focus: "not-a-uuid" }).ok, false);
  assert.equal(parseAccountPageQuery({ limit: "25" }).ok, false);

  const filters = parsed.ok ? parsed.filters : { query: "", status: "" as const, category: "" as const };
  const directory = accountDirectoryURL(filters, accountId);
  assert.equal(directory, `/accounts?q=ACME-01&status=active&category=operating&cursor=account-next&focus=${accountId}`);
  assert.equal(accountDetailURL(accountId, filters), `/accounts/${accountId}?return_to=${encodeURIComponent(directory)}`);
  assert.deepEqual(accountFiltersFromReturnPath(directory), filters);
  assert.deepEqual(accountFiltersFromReturnPath("/accounts?status=active&status=closed"), { query: "", status: "", category: "" });
  assert.equal(parseAccountBFFSearchParams(new URLSearchParams("q=ACME-01&status=active&category=operating&cursor=account-next&limit=25")).ok, true);
  assert.equal(parseAccountBFFSearchParams(new URLSearchParams("focus=10000000-0000-4000-8000-000000000001&limit=25")).ok, false);
  assert.equal(parseAccountBFFSearchParams(new URLSearchParams("status=paused&limit=25")).ok, false);
  assert.equal(parseAccountHistoryBFFSearchParams(new URLSearchParams("cursor=history-next&limit=25")).ok, true);
  assert.equal(parseAccountHistoryBFFSearchParams(new URLSearchParams("status=posted&limit=25")).ok, false);
});
