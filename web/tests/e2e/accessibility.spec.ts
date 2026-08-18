import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

import { mockOperatorConsole } from "./fixtures";

test("the operator console has no automatically detectable accessibility violations", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Balance clarity, without guesswork" })).toBeVisible();
  const results = await new AxeBuilder({ page }).analyze();
  expect(results.violations).toEqual([]);
});
