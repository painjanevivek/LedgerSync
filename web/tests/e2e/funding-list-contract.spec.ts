import { expect, test, type Route } from "@playwright/test";

import { fundingEvent, mockOperatorConsole } from "./fixtures";

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
}

test.beforeEach(async ({ page }) => {
  await mockOperatorConsole(page);
});

test("funding status, page count, cursor, and detail return context are URL reproducible", async ({ page }) => {
  let requestedURL = "";
  await page.unroute("**/api/funding-events?*");
  await page.route("**/api/funding-events?*", (route) => {
    requestedURL = route.request().url();
    const cursor = new URL(requestedURL).searchParams.get("cursor");
    return json(route, { events: [fundingEvent], next_cursor: cursor ? "" : "funding-next" });
  });

  await page.goto("/funding");
  await page.getByText("Filter records", { exact: true }).click();
  await page.getByLabel("Exact funding status").selectOption("requested");
  await page.getByRole("button", { name: "Apply filters" }).click();
  await expect(page).toHaveURL(/\/funding\?status=requested$/);
  await expect.poll(() => requestedURL).toContain("status=requested");
  await expect(page.getByText("1 record on this page. A total is not calculated or implied.")).toBeVisible();
  await expect(page.getByRole("link", { name: "Open record" })).toHaveAttribute("href", new RegExp(`return_to=${encodeURIComponent("/funding?status=requested")}`));
  await page.getByRole("link", { name: "Next page" }).click();
  await expect(page).toHaveURL(/status=requested&cursor=funding-next/);
  await expect.poll(() => requestedURL).toContain("cursor=funding-next");
  await page.getByRole("button", { name: "Clear all" }).click();
  await expect(page).toHaveURL(/\/funding$/);
  await expect(page.getByLabel("Exact funding status")).toHaveValue("");
});

test("invalid funding URLs do not request funding or account evidence", async ({ page }) => {
  let fundingRequested = false;
  let accountsRequested = false;
  await page.unroute("**/api/funding-events?*");
  await page.unroute("**/api/me/accounts?*");
  await page.route("**/api/funding-events?*", (route) => { fundingRequested = true; return json(route, {}, 500); });
  await page.route("**/api/me/accounts?*", (route) => { accountsRequested = true; return json(route, {}, 500); });

  await page.goto("/funding?status=requested&status=posted");

  await expect(page.getByText("Invalid funding investigation URL")).toBeVisible();
  expect(fundingRequested).toBe(false);
  expect(accountsRequested).toBe(false);
});
