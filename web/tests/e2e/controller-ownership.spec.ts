import { expect, test, type Page } from "@playwright/test";

import { mockOperatorConsole, sourceAccount, transfer } from "./fixtures";

function captureAPIPaths(page: Page) {
  const paths: string[] = [];
  page.on("request", (request) => {
    const path = new URL(request.url()).pathname;
    if (path.startsWith("/api/")) paths.push(path);
  });
  return paths;
}

test("account controller does not start transfer, reconciliation, or orientation graphs", async ({ page }) => {
  await mockOperatorConsole(page);
  const paths = captureAPIPaths(page);
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  await expect(page.getByRole("heading", { name: sourceAccount.display_name })).toBeVisible();

  expect(paths).toContain("/api/session");
  expect(paths).toContain("/api/me/accounts");
  expect(paths.some((path) => path.startsWith("/api/transfers"))).toBe(false);
  expect(paths.some((path) => path.startsWith("/api/reconciliation"))).toBe(false);
  expect(paths.some((path) => path.startsWith("/api/local/orientation"))).toBe(false);
});

test("transfer detail controller owns only account-picker and transfer evidence", async ({ page }) => {
  await mockOperatorConsole(page);
  const paths = captureAPIPaths(page);
  await page.goto(`/transfers/${transfer.transfer_id}`);
  await expect(page.getByRole("heading", { name: "Transfer detail", exact: true })).toBeVisible();

  await expect.poll(() => paths).toContain("/api/me/accounts");
  await expect.poll(() => paths).toContain(`/api/transfers/${transfer.transfer_id}`);
  await expect.poll(() => paths).toContain(`/api/transfers/${transfer.transfer_id}/explainability`);
  expect(paths.some((path) => path.startsWith("/api/reconciliation"))).toBe(false);
  expect(paths.some((path) => path.startsWith("/api/local/orientation"))).toBe(false);
});

test("reconciliation controller does not load account or transfer graphs", async ({ page }) => {
  await mockOperatorConsole(page);
  const paths = captureAPIPaths(page);
  await page.goto("/reconciliation");
  await expect(page.getByRole("heading", { name: "Reconciliation", exact: true })).toBeVisible();

  await expect.poll(() => paths).toContain("/api/reconciliation/runs");
  expect(paths.some((path) => path.startsWith("/api/me/accounts"))).toBe(false);
  expect(paths.some((path) => path.startsWith("/api/transfers"))).toBe(false);
  expect(paths.some((path) => path.startsWith("/api/local/orientation"))).toBe(false);
});

test("guide controller has no financial or operational evidence graph", async ({ page }) => {
  await mockOperatorConsole(page);
  const paths = captureAPIPaths(page);
  await page.goto("/guide");
  await expect(page.getByRole("heading", { name: "Use LedgerSync step by step" })).toBeVisible();

  expect(paths).toEqual(["/api/session"]);
});
