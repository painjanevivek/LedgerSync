import { expect, test, type Route } from "@playwright/test";

import { mockOperatorConsole, sourceAccount, transfer } from "./fixtures";

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
}

test.beforeEach(async ({ page }) => {
  await mockOperatorConsole(page);
});

test("transfer filters, cursor, export scope, and detail return context share one URL contract", async ({ page }) => {
  let requestedURL = "";
  await page.unroute("**/api/transfers?*");
  await page.route("**/api/transfers?*", (route) => {
    requestedURL = route.request().url();
    const cursor = new URL(requestedURL).searchParams.get("cursor");
    return json(route, { transfers: [transfer], next_cursor: cursor ? "" : "transfer-next" });
  });

  await page.goto("/transfers");
  await page.getByLabel("Search transfers").fill("ABC-1");
  await page.getByLabel("Exact account ID").fill(sourceAccount.account_id);
  await page.getByLabel("Financial status").selectOption("pending");
  await page.getByLabel("From date (UTC)").fill("2026-08-01");
  await page.getByLabel("To date (UTC)").fill("2026-08-31");
  await page.getByRole("button", { name: "Apply filters" }).click();

  await expect.poll(() => new URL(page.url()).searchParams.get("q")).toBe("abc-1");
  const visible = new URL(page.url()).searchParams;
  expect(visible.get("accountId")).toBe(sourceAccount.account_id);
  expect(visible.get("status")).toBe("pending");
  expect(visible.get("from")).toBe("2026-08-01T00:00:00.000Z");
  expect(visible.get("to")).toBe("2026-08-31T23:59:59.999Z");
  await expect.poll(() => requestedURL).toContain("accountId=");
  await expect(page.getByText(/1 record on this page\. A total is not calculated or implied\./)).toBeVisible();
  await expect(page.getByRole("link", { name: "Open record" })).toHaveAttribute("href", new RegExp(`return_to=${encodeURIComponent(`/transfers?${visible}`)}`));

  await page.getByRole("button", { name: "Export transfer details" }).click();
  await expect(page.getByText(`Search: abc-1 · Account ID: ${sourceAccount.account_id} · Financial status: pending · From UTC: 2026-08-01T00:00:00.000Z · To UTC: 2026-08-31T23:59:59.999Z`)).toBeVisible();
  await page.getByRole("button", { name: "Cancel" }).click();

  await page.getByRole("link", { name: "Next page" }).click();
  await expect(page).toHaveURL(/cursor=transfer-next/);
  await expect.poll(() => requestedURL).toContain("cursor=transfer-next");
});

test("invalid transfer URLs do not request transfer or account-picker evidence", async ({ page }) => {
  let transfersRequested = false;
  let accountsRequested = false;
  await page.unroute("**/api/transfers?*");
  await page.unroute("**/api/me/accounts?*");
  await page.route("**/api/transfers?*", (route) => { transfersRequested = true; return json(route, {}, 500); });
  await page.route("**/api/me/accounts?*", (route) => { accountsRequested = true; return json(route, {}, 500); });

  await page.goto("/transfers?status=posted&status=rejected");

  await expect(page.getByText("Invalid transfer investigation URL")).toBeVisible();
  expect(transfersRequested).toBe(false);
  expect(accountsRequested).toBe(false);
});
