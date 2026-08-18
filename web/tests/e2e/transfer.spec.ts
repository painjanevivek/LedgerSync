import { expect, test } from "@playwright/test";

import { mockOperatorConsole } from "./fixtures";

test("an authorized operator retries a lost response with the same idempotency key", async ({ page }) => {
  await mockOperatorConsole(page);
  const keys: string[] = [];
  let attempts = 0;
  await page.route("**/api/transfers", async (route) => {
    keys.push(route.request().headers()["idempotency-key"] ?? "");
    attempts += 1;
    if (attempts === 1) return route.abort("failed");
    return route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify({ transfer_id: "transfer-replayed" }) });
  });

  await page.goto("/");
  await page.getByRole("button", { name: "Transfers" }).click();
  await page.getByLabel("Amount").fill("12.50");
  await page.getByRole("button", { name: "Post internal transfer" }).click();
  await expect(page.getByText("Result not yet confirmed")).toBeVisible();
  await page.getByRole("button", { name: "Retry same transfer" }).click();
  await expect(page.getByText("Transfer posted", { exact: true })).toBeVisible();
  expect(keys).toHaveLength(2);
  expect(keys[0]).toBeTruthy();
  expect(keys[1]).toBe(keys[0]);
});

test("a malformed exact-money amount is rejected before it reaches the API", async ({ page }) => {
  await mockOperatorConsole(page);
  let calls = 0;
  await page.route("**/api/transfers", async (route) => { calls += 1; await route.fulfill({ status: 500, body: "{}" }); });

  await page.goto("/transfers");
  await page.getByLabel("Amount").fill("1.999");
  await page.getByRole("button", { name: "Post internal transfer" }).click();
  await expect(page.getByText("USD supports at most 2 decimal places.")).toBeVisible();
  expect(calls).toBe(0);
});

test("an insufficient-funds outcome clearly states that no movement occurred", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/transfers", (route) => route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ error: { code: "insufficient_funds" } }) }));
  await page.goto("/transfers");
  await page.getByLabel("Amount").fill("999.00");
  await page.getByRole("button", { name: "Post internal transfer" }).click();
  await expect(page.getByText("Transfer rejected — insufficient posted balance. No money moved.")).toBeVisible();
});
