import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

import { mockOperatorConsole } from "./fixtures";

test("saved operational views open current evidence and support audited preference maintenance", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/search");

  const panel = page.getByRole("region", { name: "Saved operational views" });
  await expect(panel.getByText("Dead delivery events")).toBeVisible();
  await expect(panel.getByRole("link", { name: /Open current evidence/u })).toHaveAttribute("href", "/events?state=dead");
  await expect(panel).toContainText("state: dead");

  await panel.getByRole("button", { name: "Rename" }).click();
  await panel.getByLabel("New saved view name").fill("Dead events requiring review");
  await panel.getByRole("button", { name: "Save name" }).click();
  await expect(panel.getByText("Dead events requiring review")).toBeVisible();

  await panel.getByRole("button", { name: "Delete" }).click();
  const dialog = page.getByRole("dialog", { name: "Delete this saved view?" });
  await expect(dialog).toContainText("does not delete or change any ledger");
  await dialog.getByRole("button", { name: "Delete saved view" }).click();
  await expect(panel.getByText("Dead events requiring review")).toHaveCount(0);
  await expect(panel.getByRole("heading", { name: "No saved views yet" })).toBeVisible();
});

test("filter capture sends only the versioned allowlisted definition and never uses browser storage", async ({ page }) => {
  await mockOperatorConsole(page);
  let submitted: unknown;
  page.on("request", (request) => {
    if (new URL(request.url()).pathname === "/api/investigation/saved-views" && request.method() === "POST") submitted = request.postDataJSON();
  });
  await page.goto("/events?state=dead");

  await page.getByText("Save current filters").click();
  await page.getByLabel("Saved view name").fill("Dead events today");
  await page.getByRole("button", { name: "Save view" }).click();
  await expect(page.locator(".saved-view-success")).toContainText("Saved “Dead events today”");
  expect(submitted).toEqual({ name: "Dead events today", filter_schema_version: "1", domain: "events", filters: { state: "dead" } });
  expect(await page.evaluate(() => ({ local: localStorage.length, session: sessionStorage.length }))).toEqual({ local: 0, session: 0 });

  const accessibility = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
  expect(accessibility.violations).toEqual([]);
});
