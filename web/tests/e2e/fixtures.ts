import type { Page, Route } from "@playwright/test";

export const sourceAccount = { account_id: "11111111-1111-4111-8111-111111111111", display_name:"Operating Reserve", category:"operating", external_reference:"OPS-RESERVE", currency: "USD", status: "active", available_minor: "125000", ledger_minor: "125000", version: "8", as_of: "2026-08-19T12:00:00Z" };
export const destinationAccount = { account_id: "22222222-2222-4222-8222-222222222222", display_name:"Customer Funds", category:"customer_funds", external_reference:"CUSTOMER-FUNDS", currency: "USD", status: "active", available_minor: "25000", ledger_minor: "25000", version: "4", as_of: "2026-08-19T12:00:00Z" };
export const transfer = { transfer_id:"33333333-3333-4333-8333-333333333333",source_account_id:sourceAccount.account_id,destination_account_id:destinationAccount.account_id,amount_minor:"500",currency:"USD",financial_status:"posted",delivery_status:"retrying",created_at:"2026-08-19T11:00:00Z",completed_at:"2026-08-19T11:00:01Z",journal_transaction_id:"44444444-4444-4444-8444-444444444444" };
export const run = { run_id:"55555555-5555-4555-8555-555555555555",status:"matched",correlation_id:"66666666-6666-4666-8666-666666666666",scope:"All authorized USD accounts",ledger_watermark:"8",application_version:"test",schema_version:"000008",checked_account_count:"2",posting_count:"2",mismatch_count:"0",started_at:"2026-08-19T11:59:58Z",completed_at:"2026-08-19T12:00:00Z" };

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
}

export async function mockOperatorConsole(page: Page) {
  await page.route("**/api/session", (route) => json(route, { subject_id: "operator-1", tenant_id: "tenant-1", csrf_token: "csrf-test-token", scopes: ["transfers:write"], environment:"demo",tenant_label:"Meridian Labs · Test",operator_label:"Test operator" }));
  await page.route("**/api/me/accounts?*", (route) => json(route, { accounts: [sourceAccount, destinationAccount], next_cursor: "" }));
  await page.route(/\/api\/accounts\/[^/?]+(?:\?.*)?$/, (route) => {
    const account = route.request().url().includes(sourceAccount.account_id) ? sourceAccount : destinationAccount;
    return json(route, account);
  });
  await page.route("**/api/accounts/*/balance", (route) => {
    const account = route.request().url().includes(sourceAccount.account_id) ? sourceAccount : destinationAccount;
    return json(route, account);
  });
  await page.route("**/api/accounts/*/transactions?*", (route) => json(route, { transactions: [{ transfer_id: "transfer-existing", direction: "credit", amount: "500", currency: "USD", status: "posted", occurred_at: "2026-08-19T11:00:00Z" }] }));
  await page.route("**/api/transfers?*", (route) => json(route, { transfers:[transfer],next_cursor:"" }));
  await page.route("**/api/transfers/*", (route) => json(route, { ...transfer,actor_subject_id:"operator-1",postings:[{posting_id:"posting-1",account_id:sourceAccount.account_id,direction:"debit",amount_minor:"500",currency:"USD",occurred_at:transfer.completed_at},{posting_id:"posting-2",account_id:destinationAccount.account_id,direction:"credit",amount_minor:"500",currency:"USD",occurred_at:transfer.completed_at}],timeline:[] }));
  await page.route("**/api/reconciliation/runs?*", (route) => json(route, { runs:[run],next_cursor:"" }));
  await page.route("**/api/reconciliation/runs/*", (route) => json(route, run));
}
