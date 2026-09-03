import { expect, test } from "@playwright/test";

import { mockOperatorConsole, transfer } from "./fixtures";

test("canonical detail pages progressively disclose bounded deterministic relationships", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto(`/transfers/${transfer.transfer_id}`);
  const rail = page.getByRole("region", { name: "Related evidence" });
  await expect(rail).toContainText("2 explicit relationships");
  await expect(rail).toContainText("Journal transaction");
  await expect(rail.getByRole("link", { name: "Open related evidence" })).toHaveCount(1);
  await expect(rail).toContainText("No released detail route");
  await expect(rail).toContainText("PostgreSQL relationship snapshot");
});

test("relationship failure is explicit and never becomes a verified empty state", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.unroute("**/api/investigation/related/**");
  await page.route("**/api/investigation/related/**", (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "temporary_unavailable" } }) }));
  await page.goto(`/transfers/${transfer.transfer_id}`);
  await expect(page.getByRole("heading", { name: "Related evidence unavailable" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "No explicit related evidence" })).toHaveCount(0);
});
