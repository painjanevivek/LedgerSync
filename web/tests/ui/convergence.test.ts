import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { ConsoleFooter } from "../../src/features/console/ConsoleShell";
import { EvidenceFreshness } from "../../src/ui/display/Evidence";
import { FundingRequestFlow } from "../../src/features/funding/FundingRequestFlow";
import { TransferList } from "../../src/features/transfers/TransferViews";

test("console footer provides the guide without adding unverified financial or infrastructure claims", () => {
  const markup = renderToStaticMarkup(createElement(ConsoleFooter));
  assert.match(markup, /LedgerSync · Internal ledger/);
  assert.match(markup, /href="\/guide"/);
  assert.match(markup, /Help and guide/);
  assert.doesNotMatch(markup, /PostgreSQL|Redis|Cached reads|version-checked|Inc\./);
});

test("evidence freshness distinguishes verified, refreshing, and historical facts", () => {
  const current = renderToStaticMarkup(createElement(EvidenceFreshness, { state: "current", verifiedAt: "2026-08-25T10:00:00Z", label: "Balance evidence" }));
  const refreshing = renderToStaticMarkup(createElement(EvidenceFreshness, { state: "refreshing", verifiedAt: "2026-08-25T10:00:00Z", label: "Balance evidence" }));
  const historical = renderToStaticMarkup(createElement(EvidenceFreshness, { state: "historical", verifiedAt: "2026-08-25T10:00:00Z", label: "Balance evidence", reason: "Refresh failed." }));
  assert.match(current, /current/);
  assert.match(current, /Balance evidence verified/);
  assert.match(refreshing, /Refreshing; prior balance evidence verified/);
  assert.match(historical, /historical/);
  assert.match(historical, /not refreshed/);
  assert.match(historical, /Refresh failed/);
});

test("overview recent-transfer variant never claims the end of history", () => {
  const markup = renderToStaticMarkup(createElement(TransferList, {
    variant: "recent",
    transfers: [{
      transfer_id: "11111111-1111-4111-8111-111111111111",
      source_account_id: "22222222-2222-4222-8222-222222222222",
      destination_account_id: "33333333-3333-4333-8333-333333333333",
      amount_minor: "1250",
      currency: "INR",
      financial_status: "posted",
      delivery_status: "delivered",
      created_at: "2026-08-25T09:59:59Z",
      completed_at: "2026-08-25T10:00:00Z",
    }],
  }));
  assert.match(markup, /Recent transfer records/);
  assert.match(markup, /Latest five loaded records/);
  assert.match(markup, /View all transfers/);
  assert.doesNotMatch(markup, /End of available records/);
});

test("converged controls, boundaries, and reduced motion use the shared design contract", () => {
  const tokens = readFileSync(new URL("../../src/styles/tokens.css", import.meta.url), "utf8");
  const buttons = readFileSync(new URL("../../src/styles/primitives/buttons.css", import.meta.url), "utf8");
  const tables = readFileSync(new URL("../../src/styles/primitives/tables.css", import.meta.url), "utf8");
  const responsive = readFileSync(new URL("../../src/styles/layout/responsive-shell.css", import.meta.url), "utf8");
  assert.match(tokens, /--line-strong:\s*#[0-9a-f]{6}/i);
  assert.match(buttons, /\.button\s*\{[^}]*min-height:\s*var\(--target-compact\)/);
  assert.match(tables, /\.data-table\s*\{[^}]*font-size:\s*var\(--type-body\)/);
  assert.match(responsive, /transition-duration:\s*\.01ms !important/);
  assert.match(responsive, /transition-delay:\s*0ms !important/);
});

test("funding entry fails closed when the authorized account directory is unavailable", () => {
  const markup = renderToStaticMarkup(createElement(FundingRequestFlow, {
    accounts: [],
    accountsLoading: false,
    accountsError: "Authorized accounts could not be refreshed.",
    accountsScopeComplete: false,
    onRetryAccounts: () => undefined,
    csrfToken: "test-csrf",
    online: true,
    canWrite: true,
    open: true,
    onClose: () => undefined,
    onCreated: async () => undefined,
  }));
  assert.match(markup, /Eligible accounts unavailable/);
  assert.match(markup, /never presented as an empty one/);
  assert.doesNotMatch(markup, /<select/);
});

test("funding entry discloses an incomplete account scope instead of using a partial picker", () => {
  const markup = renderToStaticMarkup(createElement(FundingRequestFlow, {
    accounts: [],
    accountsLoading: false,
    accountsError: null,
    accountsScopeComplete: false,
    onRetryAccounts: () => undefined,
    csrfToken: "test-csrf",
    online: true,
    canWrite: true,
    open: true,
    onClose: () => undefined,
    onCreated: async () => undefined,
  }));
  assert.match(markup, /Account selection is incomplete/);
  assert.match(markup, /complete server-backed selector/);
  assert.doesNotMatch(markup, /<select/);
});
