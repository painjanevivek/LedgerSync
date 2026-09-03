import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

import { mockOperatorConsole } from "./fixtures";

const renderAttempt = "13131313-1313-4131-8131-131313131313";
const sensitiveProbeMessage = "confidential-render-probe-must-never-reach-the-browser";

test("unknown routes are non-disclosing and return safely to the overview", async ({ page }) => {
  await mockOperatorConsole(page);
  const response = await page.goto("/route-that-is-not-released");

  expect(response?.status()).toBe(404);
  await expect(page.getByRole("heading", { name: "Page unavailable" })).toBeVisible();
  await expect(page.getByText("No record or access status is disclosed.")).toBeVisible();
  await expect(page.getByText(/tenant|permission|administrator|record exists/i)).toHaveCount(0);
  const accessibility = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
  expect(accessibility.violations).toEqual([]);
  await page.getByRole("link", { name: "Return to overview" }).click();
  await expect(page).toHaveURL("/");
  await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
});

test("a render failure exposes no raw error and a same-page retry recovers", async ({ page }) => {
  await mockOperatorConsole(page);
  const response = await page.goto(`/test-support/route-error?attempt=${renderAttempt}`);

  expect(response?.status()).toBe(500);
  await expect(page.getByRole("heading", { name: "This page could not be shown safely." })).toBeVisible();
  await expect(page.getByText("LedgerSync has not inferred a financial result.")).toBeVisible();
  await expect(page.getByText(sensitiveProbeMessage)).toHaveCount(0);
  await expect(page.getByText(/digest|request id|correlation id/i)).toHaveCount(0);
  const accessibility = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
  expect(accessibility.violations).toEqual([]);

  await page.getByRole("button", { name: "Try again safely" }).click();
  await expect(page.getByRole("heading", { name: "The page rendered on retry." })).toBeVisible();
  await expect(page).toHaveURL(`/test-support/route-error?attempt=${renderAttempt}`);
});

test("hidden administration and an unknown route remain indistinguishable", async ({ page }) => {
  await mockOperatorConsole(page);
  const admin = await page.goto("/admin");
  const adminCopy = await page.locator("main").innerText();
  const unknown = await page.goto("/another-unreleased-route");
  const unknownCopy = await page.locator("main").innerText();

  expect(admin?.status()).toBe(404);
  expect(unknown?.status()).toBe(404);
  expect(adminCopy).toBe(unknownCopy);
});
