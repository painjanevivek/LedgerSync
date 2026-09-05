import { expect, test } from "@playwright/test";
import { mockOperatorConsole, sourceAccount, destinationAccount } from "./fixtures";

test("changed balances require renewed confirmation before a new transfer is sent", async ({ page }) => {
  await mockOperatorConsole(page, { experienceMode: "simple" });
  let posts = 0;
  await page.route("**/api/transfers", route => { posts++; return route.abort(); });
  await page.goto("/transfers/new");
  await page.getByLabel("Amount").fill("12.50");
  await page.getByRole("button", { name: "Review transfer", exact: true }).click();
  await expect(page.getByText("Expected balances after this transfer")).toBeVisible();
  await page.route(`**/api/accounts/${sourceAccount.account_id}`, route => route.fulfill({ contentType: "application/json", body: JSON.stringify({ ...sourceAccount, available_minor: "124000", version: "9" }) }));
  await page.getByRole("button", { name: "Confirm transfer", exact: true }).click();
  await expect(page.getByText(/Account details changed/)).toBeVisible();
  expect(posts).toBe(0);
  await expect(page.locator('[data-minor-units="122750"]')).toBeVisible();
  await page.getByRole("button", { name: "Confirm transfer", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Do not create another transfer" })).toBeVisible();
  expect(posts).toBe(1);
  await expect(page.getByText("Expected balances after this transfer")).toHaveCount(0);
});

test("failed browser storage blocks dispatch and does not overwrite retained requests", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.addInitScript(() => { Storage.prototype.setItem = () => { throw new DOMException("Storage blocked", "SecurityError"); }; });
  let posts = 0;
  await page.route("**/api/transfers", route => { posts++; return route.abort(); });
  await page.goto("/transfers/new");
  await page.getByLabel("Amount").fill("12.50");
  await page.getByRole("button", { name: "Review transfer", exact: true }).click();
  await page.getByRole("button", { name: "Confirm transfer", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Transfer retry information is unavailable" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Confirm transfer", exact: true })).toBeDisabled();
  expect(posts).toBe(0);
});

test("unrecognized retained request information fails closed without deletion", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.addInitScript(() => sessionStorage.setItem("ledgersync.transfer.intent.tenant-1", '{"version":99,"request":"legacy"}'));
  await page.goto("/transfers");
  await page.getByRole("link", { name: "Review original request" }).click();
  await expect(page.getByRole("heading", { name: "Transfer retry information is unavailable" })).toBeVisible();
  expect(await page.evaluate(() => sessionStorage.getItem("ledgersync.transfer.intent.tenant-1"))).toBe('{"version":99,"request":"legacy"}');
});

test("confirmed completion survives a failed follow-up refresh", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/transfers", route => route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify({ transfer_id: "33333333-3333-4333-8333-333333333333", status: "posted", currency: "INR", amount_minor: "1250", occurred_at: "2026-09-05T00:00:00Z", balances: {} }) }));
  await page.route("**/api/transfers/*", route => route.abort());
  await page.route("**/api/transfers?*", route => route.abort());
  await page.goto("/transfers/new");
  await page.getByLabel("Amount").fill("12.50");
  await page.getByRole("button", { name: "Review transfer", exact: true }).click();
  await page.getByRole("button", { name: "Confirm transfer", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Transfer completed", exact: true })).toBeVisible();
  await expect(page.getByText("This result is confirmed.", { exact: false })).toBeVisible();
});

test("guided review fits all required widths and keeps risk in the main workflow", async ({ page }) => {
  await mockOperatorConsole(page, { experienceMode: "simple" });
  await page.goto("/transfers/new");
  await page.getByLabel("Amount").fill("12.50");
  await page.getByRole("button", { name: "Review transfer", exact: true }).click();
  for (const width of [320, 360, 390, 768, 1024, 1280, 1440]) {
    await page.setViewportSize({ width, height: 1058 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
    await expect(page.getByText("Expected balances after this transfer")).toBeVisible();
  }
  await page.setViewportSize({ width: 1487, height: 1058 });
  await page.evaluate(() => window.scrollTo({ top: 0, left: 0, behavior: "instant" }));
  expect(await page.locator(".skip-link").evaluate(element => ({ bottom: element.getBoundingClientRect().bottom, focused: element === document.activeElement }))).toMatchObject({ focused: false });
  expect(await page.locator(".skip-link").evaluate(element => element.getBoundingClientRect().bottom)).toBeLessThanOrEqual(0);
  await page.screenshot({ path: "../docs/design/qa/guided-transfer-review-desktop.png", fullPage: true });
  await expect(page.getByRole("navigation", { name: "Primary navigation", exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Confirm transfer", exact: true })).toBeVisible();
  expect(sourceAccount.account_id).not.toBe(destinationAccount.account_id);
});
