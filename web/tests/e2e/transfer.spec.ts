import { expect, test } from "@playwright/test";

import { mockOperatorConsole } from "./fixtures";

test("an unauthenticated visitor sees no invented financial data", async ({ page }) => {
  await page.route("**/api/session", (route) => route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ error: { code: "unauthorized" } }) }));

  await page.goto("/");

  await expect(page.getByRole("heading", { name: "Operator workspace unavailable" })).toBeVisible();
  await expect(page.getByText("No authorized session")).toBeVisible();
  await expect(page.getByText("12,458,974.21")).toHaveCount(0);
});

test("an authorized operator retries a lost response with the same idempotency key", async ({ page }) => {
  await mockOperatorConsole(page);
  const keys: string[] = [];
  let attempts = 0;
  await page.route("**/api/transfers", async (route) => {
    keys.push(route.request().headers()["idempotency-key"] ?? "");
    attempts += 1;
    if (attempts === 1) return route.abort("failed");
    return route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify({ transfer_id: "transfer-replayed", status: "posted", currency: "INR", amount_minor: "1250", occurred_at: "2026-08-19T12:00:01Z", minimum_balance_versions: {}, balances: {} }) });
  });

  await page.goto("/");
  await page.getByRole("link", { name: "Transfers", exact: true }).click();
  await page.getByLabel("Amount").fill("12.50");
  await page.getByRole("button", { name: "Review transfer" }).click();
  await page.getByRole("button", { name: "Confirm and post" }).click();
  await expect(page.getByText("Result not yet confirmed")).toBeVisible();
  await page.getByRole("button", { name: "Retry same transfer" }).click();
  await expect(page.getByText("Transfer posted", { exact: true })).toBeVisible();
  expect(keys).toHaveLength(2);
  expect(keys[0]).toBeTruthy();
  expect(keys[1]).toBe(keys[0]);
});

test("an unknown transfer survives reload with its exact intent and only the same-key retry action", async ({ page }) => {
  await mockOperatorConsole(page);
  const keys: string[] = [];
  let attempts = 0;
  await page.route("**/api/transfers", async (route) => {
    keys.push(route.request().headers()["idempotency-key"] ?? "");
    attempts += 1;
    if (attempts === 1) return route.abort("failed");
    return route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify({ transfer_id: "transfer-restored", status: "posted", currency: "INR", amount_minor: "1250", occurred_at: "2026-08-19T12:00:01Z", minimum_balance_versions: {}, balances: {} }) });
  });

  await page.goto("/transfers");
  await page.getByLabel("Exact amount").fill("12.50");
  await page.getByRole("button", { name: "Review transfer" }).click();
  await page.getByRole("button", { name: "Confirm and post" }).click();
  await expect(page.getByText("Result not yet confirmed")).toBeVisible();

  await page.reload();
  await expect(page.getByRole("heading", { name: "Confirm exact transfer" })).toBeVisible();
  await expect(page.getByText("INR 12.50")).toBeVisible();
  await expect(page.getByText("Editing is locked until this exact outcome is confirmed.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Back to edit" })).toHaveCount(0);
  await page.getByRole("button", { name: "Retry same transfer" }).click();
  await expect(page.getByRole("heading", { name: "Transfer posted" })).toBeVisible();
  expect(keys).toHaveLength(2);
  expect(keys[0]).toBeTruthy();
  expect(keys[1]).toBe(keys[0]);
});

test("a final rejection clears its key before a genuinely new intent", async ({ page }) => {
  await mockOperatorConsole(page);
  const keys: string[] = [];
  await page.route("**/api/transfers", (route) => {
    keys.push(route.request().headers()["idempotency-key"] ?? "");
    return route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ error: { code: "insufficient_funds" } }) });
  });

  await page.goto("/transfers");
  await page.getByLabel("Exact amount").fill("999.00");
  await page.getByRole("button", { name: "Review transfer" }).click();
  await page.getByRole("button", { name: "Confirm and post" }).click();
  await page.getByRole("button", { name: "Back to edit" }).click();
  await page.getByLabel("Exact amount").fill("998.00");
  await page.getByRole("button", { name: "Review transfer" }).click();
  await page.getByRole("button", { name: "Confirm and post" }).click();
  expect(keys).toHaveLength(2);
  expect(keys[0]).toBeTruthy();
  expect(keys[1]).toBeTruthy();
  expect(keys[1]).not.toBe(keys[0]);
});

test("a posted confirmation exposes journal, UTC, and committed balance-version evidence", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockOperatorConsole(page);
  const transferId = "77777777-7777-4777-8777-777777777777";
  const journalId = "88888888-8888-4888-8888-888888888888";
  await page.route("**/api/transfers", (route) => route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify({
    transfer_id: transferId,
    status: "posted",
    currency: "INR",
    amount_minor: "1250",
    occurred_at: "2026-08-19T12:00:01Z",
    minimum_balance_versions: { "11111111-1111-4111-8111-111111111111": "9", "22222222-2222-4222-8222-222222222222": "5" },
    balances: {
      "11111111-1111-4111-8111-111111111111": { account_id: "11111111-1111-4111-8111-111111111111", currency: "INR", posted_minor: "123750", version: "9", as_of: "2026-08-19T12:00:01Z" },
      "22222222-2222-4222-8222-222222222222": { account_id: "22222222-2222-4222-8222-222222222222", currency: "INR", posted_minor: "26250", version: "5", as_of: "2026-08-19T12:00:01Z" },
    },
  }) }));
  await page.route(`**/api/transfers/${transferId}`, (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({
    transfer_id: transferId,
    financial_status: "posted",
    delivery_status: "pending",
    currency: "INR",
    amount_minor: "1250",
    source_account_id: "11111111-1111-4111-8111-111111111111",
    destination_account_id: "22222222-2222-4222-8222-222222222222",
    journal_transaction_id: journalId,
    created_at: "2026-08-19T12:00:00Z",
    completed_at: "2026-08-19T12:00:01Z",
    actor_subject_id: "operator-1",
    postings: [],
    timeline: [],
  }) }));

  await page.goto("/transfers");
  await page.getByLabel("Exact amount").fill("12.50");
  await page.getByRole("button", { name: "Review transfer" }).click();
  await page.getByRole("button", { name: "Confirm and post" }).click();
  await expect(page.getByRole("heading", { name: "Transfer posted" })).toBeVisible();
  await expect(page.getByText(journalId)).toBeVisible();
  await expect(page.getByText("INR 1237.50")).toBeVisible();
  await expect(page.getByText("version 9")).toBeVisible();
  await expect(page.getByText("INR 262.50")).toBeVisible();
  await expect(page.getByText("version 5")).toBeVisible();
  await expect(page.getByText("19 Aug 2026, 12:00:01 UTC").first()).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
});

test("a malformed exact-money amount is rejected before it reaches the API", async ({ page }) => {
  await mockOperatorConsole(page);
  let calls = 0;
  await page.route("**/api/transfers", async (route) => { calls += 1; await route.fulfill({ status: 500, body: "{}" }); });

  await page.goto("/transfers");
  const amount = page.getByLabel("Amount");
  await amount.fill("1.999");
  await page.getByRole("button", { name: "Review transfer" }).click();
  await expect(page.getByText("INR supports at most 2 decimal places.")).toBeVisible();
  await amount.fill("92233720368547758.08");
  await page.getByRole("button", { name: "Review transfer" }).click();
  await expect(page.getByText("Amount exceeds the supported exact minor-unit range.")).toBeVisible();
  expect(calls).toBe(0);
});

test("an insufficient-funds outcome clearly states that no movement occurred", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/transfers", (route) => route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ error: { code: "insufficient_funds" } }) }));
  await page.goto("/transfers");
  await page.getByLabel("Amount").fill("999.00");
  await page.getByRole("button", { name: "Review transfer" }).click();
  await page.getByRole("button", { name: "Confirm and post" }).click();
  await expect(page.getByText("Transfer rejected — insufficient posted balance. No money moved.")).toBeVisible();
});
