import type { Page, Route } from "@playwright/test";

const sourceAccount = { account_id: "11111111-1111-4111-8111-111111111111", currency: "USD", status: "active", available_minor: "125000", ledger_minor: "125000", version: "8", as_of: "2026-08-19T12:00:00Z" };
const destinationAccount = { account_id: "22222222-2222-4222-8222-222222222222", currency: "USD", status: "active", available_minor: "25000", ledger_minor: "25000", version: "4", as_of: "2026-08-19T12:00:00Z" };

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
}

export async function mockOperatorConsole(page: Page) {
  await page.route("**/api/session", (route) => json(route, { subject_id: "operator-1", tenant_id: "tenant-1", csrf_token: "csrf-test-token", scopes: ["transfers:write"] }));
  await page.route("**/api/me/accounts", (route) => json(route, { accounts: [sourceAccount, destinationAccount] }));
  await page.route("**/api/accounts/*/balance", (route) => {
    const account = route.request().url().includes(sourceAccount.account_id) ? sourceAccount : destinationAccount;
    return json(route, account);
  });
  await page.route("**/api/accounts/*/transactions?*", (route) => json(route, { transactions: [{ transfer_id: "transfer-existing", direction: "credit", amount: "500", currency: "USD", status: "posted", occurred_at: "2026-08-19T11:00:00Z" }] }));
}
