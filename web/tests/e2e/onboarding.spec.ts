import { expect, test, type Route } from "@playwright/test";

import { mockOperatorConsole } from "./fixtures";

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

test("signed-out visitors receive a deliberate login layer", async ({ page }) => {
  await page.route("**/api/session", (route) =>
    json(route, { error: { code: "unauthorized" } }, 401),
  );
  await page.goto("/");

  await expect(
    page.getByRole("heading", { name: "Your ledger starts empty." }),
  ).toBeVisible();
  await expect(page.getByRole("link", { name: "Log in" })).toHaveAttribute(
    "href",
    /\/api\/auth\/sign-in/,
  );
  await expect(page.getByText(/sample balances/i)).toBeVisible();
});

test("a new local user sees an actionable zero-data dashboard and guide", async ({
  page,
}) => {
  await mockOperatorConsole(page);
  await page.route("**/api/me/accounts?*", (route) =>
    json(route, { accounts: [], next_cursor: "" }),
  );
  await page.route("**/api/transfers?*", (route) =>
    json(route, { transfers: [], next_cursor: "" }),
  );
  await page.route("**/api/reconciliation/runs?*", (route) =>
    json(route, { runs: [], next_cursor: "" }),
  );

  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Start with four simple steps" }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Create your first account" }),
  ).toHaveAttribute("href", "/accounts/new");
  await expect(page.getByRole("link", { name: /Add a funding record/ })).toHaveAttribute("href", "/funding");
  await expect(page.getByRole("link", { name: /Make a transfer/ })).toHaveAttribute("href", "/transfers");

  await page.getByRole("link", { name: "Guide", exact: true }).click();
  await expect(page).toHaveURL(/\/guide$/);
  await expect(
    page.getByRole("heading", { name: "Use LedgerSync step by step" }),
  ).toBeVisible();
  await expect(page.getByText("Create an account", { exact: true })).toBeVisible();
  await expect(page.getByText("Check your records", { exact: true })).toBeVisible();
});
