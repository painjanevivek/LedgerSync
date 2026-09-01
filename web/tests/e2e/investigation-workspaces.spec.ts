import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

import { investigationWorkspace, mockOperatorConsole, sourceAccount } from "./fixtures";

test("investigation workspaces reopen historical context beside current evidence and support lifecycle changes", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto(`/investigations/${investigationWorkspace.investigation_id}`);

  await expect(page.getByRole("heading", { name: "Transfer delivery review" })).toBeVisible();
  const current = page.getByRole("region", { name: "Current evidence" });
  await expect(current).toContainText("Statuses and links below were re-read now");
  await expect(current).toContainText("posted");
  await expect(current).toContainText("retrying");

  const historical = page.getByRole("region", { name: "Historical investigation context" });
  await expect(historical).toContainText("does not claim those records still have the same status");
  await expect(historical).toContainText(investigationWorkspace.historical_context.query_context.value);
  await expect(historical).toContainText("created");

  await page.getByRole("button", { name: "Review evidence bundle" }).click();
  const bundleReview = page.getByRole("dialog", { name: "Review investigation bundle" });
  await expect(bundleReview).toContainText("2 identifier rows");
  await expect(bundleReview).toContainText("3 root/relationship rows");
  await expect(bundleReview).toContainText("not live financial authority");
  const download = page.waitForEvent("download");
  await bundleReview.getByRole("button", { name: "Generate audited ZIP" }).click();
  await expect(page.getByRole("heading", { name: "Bundle download started" })).toBeVisible();
  expect((await download).suggestedFilename()).toBe(`ledgersync-investigation-${investigationWorkspace.investigation_id}-20260819T120500Z-v1.zip`);

  await page.getByRole("button", { name: "Close investigation" }).click();
  await expect(page.getByRole("button", { name: "Reopen investigation" })).toBeVisible();
  await page.getByRole("button", { name: "Reopen investigation" }).click();
  await expect(page.getByRole("button", { name: "Close investigation" })).toBeVisible();

  await page.getByText("Hand off to another operator").click();
  await page.getByLabel("Recipient subject ID").fill("operator-2");
  await page.getByRole("button", { name: "Confirm ownership handoff" }).click();
  await expect(page.getByRole("heading", { name: "Investigation handed off" })).toBeVisible();

  const accessibility = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
  expect(accessibility.violations).toEqual([]);
});

test("exact search creates a server-owned workspace without copied financial fields or browser persistence", async ({ page }) => {
  await mockOperatorConsole(page);
  let submitted: Record<string, unknown> | undefined;
  page.on("request", (request) => { if (new URL(request.url()).pathname === "/api/investigation/workspaces" && request.method() === "POST") submitted = request.postDataJSON() as Record<string, unknown>; });
  await page.goto(`/search?q=${sourceAccount.account_id}`);

  const result = page.locator(".investigation-result").first();
  await result.getByText("Preserve as investigation").click();
  await result.getByLabel("Safe investigation title").fill("Account state review");
  await result.getByLabel("Taxonomy").selectOption("account_state");
  await result.getByRole("button", { name: "Create investigation workspace" }).click();

  await expect(page).toHaveURL(/\/investigations\/16161616-1616-4616-8616-161616161616$/u);
  await expect(page.getByRole("heading", { name: "Account state review" })).toBeVisible();
  expect(submitted).toEqual({ title: "Account state review", taxonomy: "account_state", query_context: { kind: "immutable_id", record_type: "account", value: sourceAccount.account_id }, root_record: { record_type: "account", record_id: sourceAccount.account_id } });
  expect(JSON.stringify(submitted)).not.toMatch(/amount|balance|payload|notes|secret|token/iu);
  expect(await page.evaluate(() => ({ local: localStorage.length, session: sessionStorage.length }))).toEqual({ local: 0, session: 0 });
});
