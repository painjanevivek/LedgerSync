import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

import { mockOperatorConsole } from "./fixtures";

test("the operator console has no automatically detectable accessibility violations", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Overview" })).toBeVisible();
  const results = await new AxeBuilder({ page }).analyze();
  expect(results.violations).toEqual([]);
});

test("compact navigation opens, closes with Escape, and restores focus", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 }); await mockOperatorConsole(page); await page.goto("/");
  const menu=page.getByRole("button",{name:/menu/i}); await menu.focus(); await menu.press("Enter"); await expect(page.getByRole("navigation",{name:"Primary navigation"})).toBeVisible(); await page.keyboard.press("Escape"); await expect(menu).toBeFocused();
});

test("forced colors and reduced motion preserve operable financial evidence", async ({ page }) => {
  await page.emulateMedia({ colorScheme:"light", reducedMotion:"reduce", forcedColors:"active" }); await mockOperatorConsole(page); await page.goto("/reconciliation");
  await expect(page.getByRole("heading",{name:"Reconciliation",exact:true})).toBeVisible(); await expect(page.getByText("Passed",{exact:true}).first()).toBeVisible();
  const results=await new AxeBuilder({page}).analyze(); expect(results.violations).toEqual([]);
});
