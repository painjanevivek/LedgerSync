import { expect, test } from "@playwright/test";
import { mockOperatorConsole, sourceAccount } from "./fixtures";

test("one account directs the operator to create a destination before a transfer", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/me/accounts?*", route => route.fulfill({ json: { accounts: [sourceAccount], next_cursor: "" } }));
  await page.route("**/api/transfers?*", route => route.fulfill({ json: { transfers: [], next_cursor: "" } }));
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Create the destination account" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Create second account" })).toHaveAttribute("href", "/accounts/new");
  await expect(page.getByRole("heading", { name: "Make your first transfer" })).toHaveCount(0);
});

test("the guide explains two accounts, independent review, and unknown recovery", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/guide");
  await page.getByText("Full operating path", { exact: true }).click();
  await expect(page.getByText(/Create a source account and a different destination/)).toBeVisible();
  await expect(page.getByText(/Ask a different authorized reviewer/)).toBeVisible();
  await page.getByText("Ledger terms", { exact: true }).click();
  await expect(page.getByText(/Retry an unknown result with the same key/)).toBeVisible();
  await expect(page.getByText(/It does not prove that a bank payment settled/)).toBeVisible();
});

test("group changes preserve filtered deep links and browser history", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/accounts?status=active");
  const nav = page.getByRole("navigation", { name: "Primary navigation" });
  await nav.getByText("Operations", { exact: true }).click();
  await nav.getByRole("link", { name: "Events & webhooks" }).click();
  await expect(page).toHaveURL(/\/events$/);
  await page.goBack();
  await expect(page).toHaveURL(/\/accounts\?status=active$/);
  await page.goForward();
  await expect(page).toHaveURL(/\/events$/);
});
