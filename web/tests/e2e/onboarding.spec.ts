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
    page.getByRole("heading", { name: "Your money workflows. One clear step at a time." }),
  ).toBeVisible();
  await expect(page.getByRole("link", { name: "Open workspace" }).first()).toHaveAttribute(
    "href",
    "/sign-in",
  );
  await expect(page.getByText("Illustrative example · No money moves.")).toBeVisible();
});

test("a new local user sees an actionable zero-data dashboard and guide", async ({
  page,
}) => {
  await mockOperatorConsole(page, { experienceMode: "simple" });
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
    page.getByRole("heading", { name: "Your money at a glance" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Start with your first account" }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Create an account" }),
  ).toHaveAttribute("href", "/accounts/new");

  await page.getByRole("link", { name: "Open the guide", exact: true }).click();
  await expect(page).toHaveURL(/\/guide$/);
  await expect(
    page.getByRole("heading", { name: "Use LedgerSync step by step" }),
  ).toBeVisible();
  await page.getByText("Reference: the six-step operating path", { exact: true }).click();
  await expect(page.getByText("Create an account", { exact: true })).toBeVisible();
  await expect(page.getByText("Check your records", { exact: true })).toBeVisible();
});
