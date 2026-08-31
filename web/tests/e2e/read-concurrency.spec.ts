import { expect, test, type Page, type Route } from "@playwright/test";

import { fundingEvent, mockOperatorConsole, sourceAccount, transfer } from "./fixtures";

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
}

async function authorizeCorrections(page: Page) {
  await page.route("**/api/session", (route) => json(route, {
    subject_id: "approver-2",
    tenant_id: "tenant-1",
    csrf_token: "csrf-test-token",
    environment: "production",
    scopes: ["corrections:read", "corrections:write", "corrections:approve"],
  }));
}

test("funding cursor navigation dispatches one URL-bound page request", async ({ page }) => {
  let cursorRequests = 0;
  const nextFunding = { ...fundingEvent, funding_event_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", external_reference: "BANK-NEXT-20260831" };
  await mockOperatorConsole(page);
  await page.route("**/api/funding-events?*", async (route) => {
    const cursor = new URL(route.request().url()).searchParams.get("cursor");
    if (!cursor) return json(route, { events: [fundingEvent], next_cursor: "opaque-funding-cursor" });
    cursorRequests += 1;
    await new Promise((resolve) => setTimeout(resolve, 100));
    return json(route, { events: [fundingEvent, nextFunding], next_cursor: "" });
  });

  await page.goto("/funding");
  await page.getByRole("link", { name: "Next page" }).click();

  await expect(page.getByText(nextFunding.external_reference, { exact: true })).toBeVisible();
  await expect(page).toHaveURL(/cursor=opaque-funding-cursor/);
  expect(cursorRequests).toBe(1);
  await expect(page.getByText(fundingEvent.external_reference, { exact: true })).toHaveCount(1);
});

test("rapid account-history pagination dispatches once and removes page overlap", async ({ page }) => {
  let cursorRequests = 0;
  const existing = { transfer_id: "transfer-existing", direction: "credit", amount: "500", currency: "INR", status: "posted", occurred_at: "2026-08-19T11:00:00Z" };
  const next = { ...existing, transfer_id: "transfer-next" };
  await mockOperatorConsole(page);
  await page.route("**/api/accounts/*/transactions?*", async (route) => {
    const cursor = new URL(route.request().url()).searchParams.get("cursor");
    if (!cursor) return json(route, { transactions: [existing], next_cursor: "opaque-history-cursor" });
    cursorRequests += 1;
    await new Promise((resolve) => setTimeout(resolve, 100));
    return json(route, { transactions: [existing, next], next_cursor: "" });
  });

  await page.goto(`/accounts/${sourceAccount.account_id}`);
  const more = page.getByRole("button", { name: "Load more" });
  await more.evaluate((button: HTMLButtonElement) => { button.click(); button.click(); });

  await expect(page.getByText(next.transfer_id, { exact: true })).toBeVisible();
  expect(cursorRequests).toBe(1);
  await expect(page.getByText(existing.transfer_id, { exact: true })).toHaveCount(1);
});

test("a later transfer refresh wins when responses resolve out of order", async ({ page }) => {
  const older = { ...transfer, transfer_id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" };
  const newer = { ...transfer, transfer_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" };
  const pending: Array<{ route: Route; release: () => void }> = [];
  let requestCount = 0;
  await mockOperatorConsole(page);
  await page.unroute("**/api/transfers?*");
  await page.route("**/api/transfers?*", (route) => {
    requestCount += 1;
    if (requestCount === 1) return json(route, { error: { code: "temporary_unavailable" } }, 503);
    return new Promise<void>((release) => pending.push({ route, release }));
  });

  await page.goto("/");
  const retry = page.getByRole("button", { name: "Retry transfers only" });
  await retry.evaluate((button: HTMLButtonElement) => { button.click(); button.click(); });
  await expect.poll(() => pending.length).toBe(2);

  await json(pending[1].route, { transfers: [newer], next_cursor: "" });
  pending[1].release();
  await expect(page.getByText(newer.transfer_id, { exact: true })).toBeVisible();
  await json(pending[0].route, { transfers: [older], next_cursor: "" });
  pending[0].release();

  await expect(page.getByText(newer.transfer_id, { exact: true })).toBeVisible();
  await expect(page.getByText(older.transfer_id, { exact: true })).toHaveCount(0);
});

test("a correction filter change rejects an in-flight cursor page", async ({ page }) => {
  const base = {
    correction_id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
    original_transfer_id: transfer.transfer_id,
    original_journal_id: transfer.journal_transaction_id,
    requester_subject_id: "requester-1",
    debit_account_id: transfer.source_account_id,
    credit_account_id: transfer.destination_account_id,
    amount_minor: transfer.amount_minor,
    currency: transfer.currency,
    reason_code: "operational_error",
    operator_note: "Verified evidence.",
    status: "approved",
    policy_version: "transfer-correction-v1",
    control_mode: "production_dual_control",
    step_up_required: true,
    approval_expires_at: "2026-09-01T12:00:00Z",
    requested_at: "2026-08-31T10:00:00Z",
    updated_at: "2026-08-31T10:00:00Z",
  };
  const requested = { ...base, correction_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", status: "requested" };
  const stalePage = { ...base, correction_id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc" };
  let pendingCursor: { route: Route; release: () => void } | undefined;
  await mockOperatorConsole(page);
  await authorizeCorrections(page);
  await page.route("**/api/transfer-corrections?*", (route) => {
    const query = new URL(route.request().url()).searchParams;
    if (query.has("cursor")) return new Promise<void>((release) => { pendingCursor = { route, release }; });
    if (query.get("status") === "requested") return json(route, { events: [requested], next_cursor: "" });
    return json(route, { events: [base], next_cursor: "opaque-correction-cursor" });
  });

  await page.goto("/corrections");
  await page.getByRole("button", { name: "Load more" }).click();
  await expect.poll(() => Boolean(pendingCursor)).toBe(true);
  await page.getByLabel("Status").selectOption("requested");
  await expect(page.getByText(requested.correction_id, { exact: true })).toBeVisible();

  await json(pendingCursor!.route, { events: [stalePage], next_cursor: "" });
  pendingCursor!.release();
  await expect(page.getByText(requested.correction_id, { exact: true })).toBeVisible();
  await expect(page.getByText(stalePage.correction_id, { exact: true })).toHaveCount(0);
});
