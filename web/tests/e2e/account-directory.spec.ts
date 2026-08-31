import { expect, test } from "@playwright/test";

import { destinationAccount, mockOperatorConsole, sourceAccount } from "./fixtures";

test("account filters and cursor context survive an object-specific investigation", async ({ page }) => {
  await mockOperatorConsole(page);
  const secondPageAccount = { ...destinationAccount, account_id: "77777777-7777-4777-8777-777777777777", display_name: "Scale account 00020", category: "operating" };
  await page.unroute("**/api/me/accounts?*");
  await page.route("**/api/me/accounts?*", (route) => {
    const url = new URL(route.request().url());
    if (url.searchParams.get("cursor")) return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ accounts: [secondPageAccount], next_cursor: "" }) });
    const filtering = url.searchParams.get("q") === "Scale" && url.searchParams.get("status") === "active" && url.searchParams.get("category") === "operating";
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ accounts: filtering ? [sourceAccount] : [sourceAccount, destinationAccount], next_cursor: filtering ? "next-account-page" : "" }) });
  });
  await page.route(/\/api\/accounts\/77777777-7777-4777-8777-777777777777(?:\?.*)?$/, (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(secondPageAccount) }));

  await page.goto("/accounts");
  await page.getByLabel("Search accounts").fill("Scale");
  await page.getByLabel("Status").selectOption("active");
  await page.getByLabel("Category").selectOption("operating");
  await page.getByRole("button", { name: "Apply filters" }).click();
  await expect(page).toHaveURL(/q=Scale.*status=active.*category=operating/);
  await expect(page.getByText("1 authorized account on this page; no total is implied.")).toBeVisible();
  await expect(page.getByText("Oldest created first")).toBeVisible();

  await page.getByRole("link", { name: "Next page" }).click();
  await expect(page).toHaveURL(/cursor=next-account-page/);
  await page.getByRole("link", { name: "Open account" }).first().click();
  await expect(page.getByRole("heading", { name: "Scale account 00020" })).toBeVisible();
  await page.getByRole("link", { name: /Back to account directory/ }).click();
  await expect(page).toHaveURL(/q=Scale.*status=active.*category=operating.*cursor=next-account-page/);
  const restoredLink = page.getByRole("region", { name: "Authorized account comparison" }).getByRole("link", { name: "Open account" });
  await expect(restoredLink).toBeFocused();
  await expect(page.getByRole("region", { name: "Authorized account comparison" }).getByText("Scale account 00020")).toBeVisible();

  await page.getByRole("link", { name: "Clear filters" }).click();
  await expect(page).toHaveURL(/\/accounts$/);
  await expect(page.getByLabel("Search accounts")).toHaveValue("");
  await expect(page.getByLabel("Status")).toHaveValue("");
  await expect(page.getByLabel("Category")).toHaveValue("");
});

test("invalid account URLs fail closed before protected directory reads", async ({ page }) => {
  let accountDirectoryRequested = false;
  await mockOperatorConsole(page);
  await page.unroute("**/api/me/accounts?*");
  await page.route("**/api/me/accounts?*", (route) => {
    accountDirectoryRequested = true;
    return route.fulfill({ status: 500, contentType: "application/json", body: "{}" });
  });

  await page.goto("/accounts?status=active&status=closed");
  await expect(page.getByText("Invalid account investigation URL")).toBeVisible();
  expect(accountDirectoryRequested).toBe(false);
  await expect(page.getByRole("link", { name: "Clear invalid filters" })).toHaveAttribute("href", "/accounts");
});

test("account directory distinguishes empty, unavailable, and offline states", async ({ page, context }) => {
  await mockOperatorConsole(page);
  await page.unroute("**/api/me/accounts?*");
  await page.route("**/api/me/accounts?*", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ accounts: [], next_cursor: "" }) }));
  await page.goto("/accounts");
  await expect(page.getByText("No accounts yet")).toBeVisible();

  await page.unroute("**/api/me/accounts?*");
  await page.route("**/api/me/accounts?*", (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "unavailable" } }) }));
  await page.reload();
  await expect(page.getByText("Accounts unavailable")).toBeVisible();

  await context.setOffline(true);
  await page.evaluate(() => window.dispatchEvent(new Event("offline")));
  await expect(page.getByText("You are offline.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Refresh accounts" })).toBeDisabled();
  await context.setOffline(false);
});

test("large exact account evidence reflows without page-level overflow", async ({ page }) => {
  await mockOperatorConsole(page);
  const longAccount = { ...sourceAccount, display_name: "International partner settlement reserve account with an intentionally long evidence label", external_reference: "PARTNER-REFERENCE-WITH-LONG-AUDIT-CONTEXT-000000000000000001", available_minor: "9223372036854775807", ledger_minor: "9223372036854775807", version: "9223372036854775807" };
  await page.unroute("**/api/me/accounts?*");
  await page.route("**/api/me/accounts?*", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ accounts: [longAccount], next_cursor: "" }) }));
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/accounts");
  await expect(page.getByText("INR 92233720368547758.07").last()).toBeVisible();
  const dimensions = await page.evaluate(() => ({ scrollWidth: document.documentElement.scrollWidth, innerWidth: window.innerWidth }));
  expect(dimensions.scrollWidth).toBeLessThanOrEqual(dimensions.innerWidth);
});
