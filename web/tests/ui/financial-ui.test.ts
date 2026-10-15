import assert from "node:assert/strict";
import test from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import type { ReconciliationRun } from "../../src/features/accounts/types";
import { FundingDetailView } from "../../src/features/funding/FundingViews";
import { ReconciliationView } from "../../src/features/reconciliation/ReconciliationViews";
import type { FundingEvent } from "../../src/lib/api/funding";
import { approvedCurrencyGroups, isAuthoritativelyReconciled, transferOutcomeLabels } from "../../src/lib/financial-ui";

test("customer funds are not silently aggregated into operating balances", () => {
  const accounts = [
    { account_id:"a",currency:"USD",status:"active" as const,available_minor:"100",ledger_minor:"100",account_version:"2",version:"1",as_of:"2026-01-01T00:00:00Z",category:"operating" as const },
    { account_id:"b",currency:"USD",status:"active" as const,available_minor:"250",ledger_minor:"250",account_version:"3",version:"1",as_of:"2026-01-01T00:00:00Z",category:"customer_funds" as const },
  ];
  assert.deepEqual(approvedCurrencyGroups(accounts), { currency:"USD", mixedCurrency:false, operatingMinor:"100", customerFundsMinor:"250" });
});

test("mixed currencies are blocked instead of silently merged", () => {
  const accounts = [
    { account_id:"a",currency:"USD",status:"active" as const,available_minor:"100",ledger_minor:"100",account_version:"2",version:"1",as_of:"2026-01-01T00:00:00Z",category:"operating" as const },
    { account_id:"b",currency:"EUR",status:"active" as const,available_minor:"250",ledger_minor:"250",account_version:"3",version:"1",as_of:"2026-01-01T00:00:00Z",category:"customer_funds" as const },
  ];
  assert.deepEqual(approvedCurrencyGroups(accounts), { currency:undefined, mixedCurrency:true, operatingMinor:"0", customerFundsMinor:"0" });
});

test("passed is permitted only for completed zero-mismatch matched evidence", () => {
  const run = { run_id:"run",status:"matched" as const,correlation_id:"c",scope:"tenant",ledger_watermark:"1",application_version:"1",schema_version:"1",checked_account_count:"1",posting_count:"2",mismatch_count:"0",started_at:"2026-01-01T00:00:00Z",completed_at:"2026-01-01T00:00:01Z" };
  assert.equal(isAuthoritativelyReconciled(run), true);
  assert.equal(isAuthoritativelyReconciled({ ...run, mismatch_count:"1" }), false);
  assert.equal(isAuthoritativelyReconciled(null), false);
});

test("financial and delivery outcomes remain separate", () => {
  assert.deepEqual(transferOutcomeLabels({ financial_status:"posted", delivery_status:"retrying" }), { financial:"posted", delivery:"Delivery retrying" });
});

function renderRun(run: ReconciliationRun) {
  return renderToStaticMarkup(createElement(ReconciliationView, {
    runs: [run],
    detail: run,
    detailRequested: true,
    error: null,
    loading: false,
    tenantId: "tenant",
    csrfToken: "csrf",
    online: true,
    canWrite: false,
    canExport: true,
    filters: {},
    onObserved: () => undefined,
    onRefresh: async () => undefined,
  }));
}

test("reconciliation detail never presents incomplete matched evidence as passed", () => {
  const run: ReconciliationRun = { run_id:"run",status:"matched",correlation_id:"c",scope:"tenant",ledger_watermark:"1",application_version:"1",schema_version:"1",checked_account_count:"1",posting_count:"2",mismatch_count:"0",started_at:"2026-01-01T00:00:00Z",completed_at:"2026-01-01T00:00:01Z" };
  for (const incomplete of [
    { ...run, completed_at: "" },
    { ...run, ledger_watermark: "" },
    { ...run, run_id: "" },
    { ...run, mismatch_count: "1" },
  ]) {
    const markup = renderRun(incomplete);
    assert.match(markup, /<h2>Result not verified<\/h2>/);
    assert.match(markup, /status-label warning/);
    assert.match(markup, /Complete run evidence is required to confirm a passing result\./);
    assert.doesNotMatch(markup, /<h2>Passed<\/h2>|status-label success/);
  }
  const verified = renderRun(run);
  assert.match(verified, /<h2>Passed<\/h2>/);
  assert.match(verified, /status-label success/);
  assert.doesNotMatch(verified, /Result not verified/);
});

test("an unreviewed funding record does not claim that production approval policy was recorded", () => {
  const event: FundingEvent = {
    funding_event_id: "abababab-abab-4bab-8bab-abababababab", destination_account_id: "22222222-2222-4222-8222-222222222222",
    external_reference: "BANK-REF-20260819", evidence_reference: "CASE-2026-0819", amount_minor: "125000", currency: "INR",
    status: "requested", requester_subject_id: "operator-1", demo_policy: false,
    requested_at: "2026-08-19T11:20:00Z", updated_at: "2026-08-19T11:20:00Z",
  };
  function renderFunding(record: FundingEvent) {
    return renderToStaticMarkup(createElement(FundingDetailView, {
      event: record, session: { subject_id: "operator-1", tenant_id: "tenant", csrf_token: "csrf", scopes: [], environment: "local" },
      reconciliation: null, loading: false, actionBusy: false, error: null, online: true, canWrite: false, canApprove: false, backHref: "/funding",
      onRefresh: async () => record, onAction: async () => false, onReconcile: () => undefined,
    }));
  }
  const unreviewed = renderFunding(event);
  assert.match(unreviewed, /Finance review required/);
  assert.match(unreviewed, /Server policy applies at review/);
  assert.doesNotMatch(unreviewed, /Production · dual control|Independent decision recorded/);
  const localApproved = renderFunding({ ...event, status: "approved", demo_policy: true, approver_subject_id: "operator-1" });
  assert.match(localApproved, /Local review recorded/);
  assert.match(localApproved, /Local workspace · single operator/);
});
