import assert from "node:assert/strict";
import test from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import type { Account, TransferDetail } from "../../src/features/accounts/types";
import { hasUnavailablePreferredDestination } from "../../src/features/transfers/TransferForm";
import { TransferEvidenceFolio, TransferHistoryState, TransferList } from "../../src/features/transfers/TransferViews";

const verifiedAt = "2026-10-10T07:30:00Z";
const source: Account = {
  account_id: "11111111-1111-4111-8111-111111111111",
  display_name: "Operating",
  currency: "INR",
  status: "active",
  available_minor: "10000",
  ledger_minor: "10000",
  account_version: "2",
  version: "1",
  as_of: verifiedAt,
};
const destination: Account = {
  ...source,
  account_id: "22222222-2222-4222-8222-222222222222",
  display_name: "Reserve",
  available_minor: "0",
  ledger_minor: "0",
};
const detail: TransferDetail = {
  transfer_id: "33333333-3333-4333-8333-333333333333",
  source_account_id: source.account_id,
  destination_account_id: destination.account_id,
  amount_minor: "1250",
  currency: "INR",
  financial_status: "posted",
  delivery_status: "retrying",
  created_at: verifiedAt,
  completed_at: verifiedAt,
  actor_subject_id: "operator",
  postings: [],
  timeline: [],
};

function renderHistory(props: Partial<Parameters<typeof TransferHistoryState>[0]> = {}) {
  return renderToStaticMarkup(createElement(TransferHistoryState, {
    recordCount: 0,
    loading: false,
    error: null,
    online: true,
    onRefresh: async () => undefined,
    ...props,
  }));
}

function renderFolio(props: Partial<Parameters<typeof TransferEvidenceFolio>[0]> = {}) {
  return renderToStaticMarkup(createElement(TransferEvidenceFolio, {
    detail: null,
    detailRequested: true,
    selectedTransferId: detail.transfer_id,
    detailError: null,
    detailLoading: false,
    accounts: [source, destination],
    explainability: null,
    explainabilityLoading: false,
    explainabilityError: null,
    online: true,
    canReadExplainability: false,
    canReadCorrections: false,
    canWriteCorrections: false,
    csrfToken: "csrf",
    onRefresh: async () => undefined,
    onRefreshExplainability: () => undefined,
    ...props,
  }));
}

test("offline or unverified transfer history does not claim a verified empty result", () => {
  for (const props of [{ online: false }, { online: false, loading: true }, {}]) {
    const markup = renderHistory(props);
    assert.match(markup, /Transfer history unavailable/);
    assert.doesNotMatch(markup, /No transfers found|Loading transfer history/);
  }
  assert.match(renderHistory({ online: false }), /An empty history has not been verified/);
});

test("only an online successful history read can report an empty filtered result", () => {
  assert.match(renderHistory({ loading: true }), /Loading transfer history/);
  assert.match(renderHistory({ verifiedAt }), /No transfers found/);
  assert.match(renderHistory({ verifiedAt }), /verified authorized scope and active filters/);
  assert.equal(renderHistory({ recordCount: 1 }), "");
  assert.equal(renderHistory({ error: "Request failed" }), "");
});

test("offline transfer detail has an unavailable state instead of indefinite loading", () => {
  const markup = renderFolio({ online: false, detailLoading: true });
  assert.match(markup, /Transfer details unavailable/);
  assert.match(markup, /Connect to load/);
  assert.doesNotMatch(markup, /Loading transfer details/);
});

test("a failed detail refresh keeps immutable outcomes and marks the retained read historical", () => {
  const markup = renderFolio({ detail, detailVerifiedAt: verifiedAt, detailError: "Request failed" });
  assert.match(markup, /Transfer details not refreshed/);
  assert.match(markup, /Shown posting and delivery states come from the last successful read/);
  assert.match(markup, /evidence-freshness historical/);
  assert.match(markup, /last verified/);
  assert.match(markup, /Money is posted; delivery is retrying/);
  assert.doesNotMatch(markup, /Transfer not posted|status-label danger/);
});

test("retained detail is historical while offline and supports the previous view context", () => {
  const markup = renderFolio({ detail, detailVerifiedAt: verifiedAt, online: false, returnTo: "/search?q=receipt" });
  assert.match(markup, /evidence-freshness historical/);
  assert.match(markup, /Back to previous view/);
  assert.match(markup, /href="\/search\?q=receipt"/);
  assert.match(markup, /<button[^>]*disabled=""[^>]*>[\s\S]*?Retry this transfer only<\/button>/);
});

test("account route links expose their name and complete account ID", () => {
  const markup = renderToStaticMarkup(createElement(TransferList, {
    transfers: [detail],
    accounts: [source, destination],
  }));
  assert.match(markup, new RegExp(`aria-label="Source account Operating · ${source.account_id}"`));
  assert.match(markup, new RegExp(`aria-label="Destination account Reserve · ${destination.account_id}"`));
});

test("a requested destination cannot fall back to a different active account", () => {
  assert.equal(hasUnavailablePreferredDestination([source, destination], destination.account_id, false), false);
  assert.equal(hasUnavailablePreferredDestination([source], destination.account_id, false), true);
  for (const unavailable of [
    { ...destination, status: "frozen" as const },
    { ...destination, status: "closed" as const },
    { ...destination, currency: "USD" },
  ]) {
    assert.equal(hasUnavailablePreferredDestination([source, unavailable], destination.account_id, false), true);
  }
  assert.equal(hasUnavailablePreferredDestination([source], undefined, false), false);
  assert.equal(hasUnavailablePreferredDestination([source], destination.account_id, true), false);
});
