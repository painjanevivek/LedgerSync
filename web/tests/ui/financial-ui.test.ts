import assert from "node:assert/strict";
import test from "node:test";

import { approvedCurrencyGroups, isAuthoritativelyReconciled, transferOutcomeLabels } from "../../src/lib/financial-ui";

test("customer funds are not silently aggregated into operating balances", () => {
  const accounts = [
    { account_id:"a",currency:"USD",status:"active" as const,available_minor:"100",ledger_minor:"100",version:"1",as_of:"2026-01-01T00:00:00Z",category:"operating" as const },
    { account_id:"b",currency:"USD",status:"active" as const,available_minor:"250",ledger_minor:"250",version:"1",as_of:"2026-01-01T00:00:00Z",category:"customer_funds" as const },
  ];
  assert.deepEqual(approvedCurrencyGroups(accounts), { currency:"USD", mixedCurrency:false, operatingMinor:"100", customerFundsMinor:"250" });
});

test("mixed currencies are blocked instead of silently merged", () => {
  const accounts = [
    { account_id:"a",currency:"USD",status:"active" as const,available_minor:"100",ledger_minor:"100",version:"1",as_of:"2026-01-01T00:00:00Z",category:"operating" as const },
    { account_id:"b",currency:"EUR",status:"active" as const,available_minor:"250",ledger_minor:"250",version:"1",as_of:"2026-01-01T00:00:00Z",category:"customer_funds" as const },
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
