import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page, type Route } from "@playwright/test";

import { investigationSearchPage, mockOperatorConsole, sourceAccount, transfer } from "./fixtures";

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
}

async function useSession(page: Page, scopes: string[]) {
  await page.route("**/api/session", (route) => json(route, {
    subject_id: "search-operator",
    tenant_id: "tenant-1",
    csrf_token: "csrf-test-token",
    scopes,
    environment: "local",
    tenant_label: "Investigation workspace",
    operator_label: "Scoped investigator",
  }));
}

test("keyboard navigation opens exact search and typed locators lead to canonical detail", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/accounts");
  const searchLink = page.getByRole("navigation", { name: "Primary navigation" }).getByRole("link", { name: "Search records" });
  await searchLink.focus();
  await expect(searchLink).toBeFocused();
  await searchLink.press("Enter");
  await expect(page).toHaveURL(/\/search$/u);

  await page.getByLabel("Exact ID or approved reference").fill(sourceAccount.account_id);
  await page.getByRole("button", { name: "Search exact evidence" }).click();
  await expect(page).toHaveURL(new RegExp(`/search\\?q=${sourceAccount.account_id}$`, "u"));
  await expect(page.getByText("2 authorized locators")).toBeVisible();
  await expect(page.getByRole("link", { name: "Open authoritative detail" })).toHaveAttribute("href", `/accounts/${sourceAccount.account_id}`);
  await expect(page.getByRole("link", { name: "Open referenced evidence" })).toHaveAttribute("href", `/transfers/${transfer.transfer_id}`);
  await expect(page.getByText("PostgreSQL · search snapshot").first()).toBeVisible();
  const renderedLocators = await page.locator(".investigation-results").textContent();
  expect(renderedLocators).not.toMatch(/125000|balance_minor|available_minor|payload|token/iu);
});

test("invalid partial lookup and missing scope make no protected search request", async ({ page }) => {
  await mockOperatorConsole(page);
  const requested: string[] = [];
  page.on("request", (request) => { if (new URL(request.url()).pathname === "/api/investigation/search") requested.push(request.url()); });
  await page.goto("/search?q=short");
  await expect(page.getByText("Invalid search URL")).toBeVisible();
  expect(requested).toEqual([]);

  await useSession(page, ["accounts:read"]);
  await page.goto(`/search?q=${sourceAccount.account_id}`);
  await expect(page.getByText("Investigation authority required")).toBeVisible();
  expect(requested).toEqual([]);
});

test("empty and unavailable search outcomes remain distinct and non-inferential", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/investigation/search?*", (route) => json(route, { ...investigationSearchPage, results: [] }));
  await page.goto(`/search?q=${sourceAccount.account_id}`);
  await expect(page.getByText("No authorized match")).toBeVisible();
  await expect(page.getByText(/does not distinguish a missing record/iu)).toBeVisible();

  await page.route("**/api/investigation/search?*", (route) => json(route, { error: { code: "temporary_unavailable" } }, 503));
  await page.reload();
  await expect(page.getByText("Search evidence unavailable")).toBeVisible();
  await expect(page.getByText(/No missing record is inferred/iu)).toBeVisible();
});

test("compact exact search reflows and passes automated accessibility checks", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 640 });
  await mockOperatorConsole(page);
  await page.goto(`/search?q=${sourceAccount.account_id}`);
  await expect(page.getByText("2 authorized locators")).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
  const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
  expect(results.violations).toEqual([]);
});
