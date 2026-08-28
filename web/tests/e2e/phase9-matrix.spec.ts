import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

import { deliveryEvent, mockOperatorConsole, run, sourceAccount, transfer } from "./fixtures";

const routes = [
  "/",
  "/accounts",
  `/accounts/${sourceAccount.account_id}`,
  "/accounts/new",
  "/transfers",
  `/transfers/${transfer.transfer_id}`,
  "/reconciliation",
  `/reconciliation/${run.run_id}`,
  "/local-status",
  "/events",
  `/events/${deliveryEvent.event_id}`,
  "/developer",
  "/recovery",
] as const;

const viewports = [
  { width: 320, height: 800 },
  { width: 390, height: 844 },
  { width: 768, height: 1024 },
  { width: 1024, height: 768 },
  { width: 1366, height: 768 },
  { width: 1440, height: 900 },
  { width: 1920, height: 1080 },
] as const;

for (const viewport of viewports) {
  test(`every populated route reflows at ${viewport.width}x${viewport.height}`, async ({ page }) => {
    await page.setViewportSize(viewport);
    await mockOperatorConsole(page);

    for (const path of routes) {
      await page.goto(path);
      await expect(page.locator("main")).toHaveCount(1);
      await expect(page.locator("main h1:visible")).toHaveCount(1);

      const reflow = await page.evaluate(() => {
        const root = document.documentElement;
        const horizontalOverflow = root.scrollWidth > root.clientWidth;
        const offenders = horizontalOverflow
          ? [...document.querySelectorAll<HTMLElement>("body *")]
              .filter((element) => !element.closest(".data-table-wrap") && !["svg", "path"].includes(element.tagName.toLowerCase()) && element.getBoundingClientRect().right > root.clientWidth + 1)
              .map((element) => `${element.tagName.toLowerCase()}.${element.className}:${Math.round(element.getBoundingClientRect().right)}`)
              .slice(0, 8)
          : [];
        return {
          horizontalOverflow,
          overflowPixels: root.scrollWidth - root.clientWidth,
          offenders,
        };
      });

      expect(reflow, `${path} at ${viewport.width}x${viewport.height}`).toEqual({
        horizontalOverflow: false,
        overflowPixels: 0,
        offenders: [],
      });
    }
  });
}

test("every populated route passes the complete authored-color WCAG A and AA rules", async ({ page }) => {
  await mockOperatorConsole(page);

  for (const path of routes) {
    await page.goto(path);
    await expect(page.locator("main h1:visible")).toHaveCount(1);
    const results = await new AxeBuilder({ page })
      .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
      .analyze();
    expect(results.violations, `${path}: ${results.violations.map((violation) => violation.id).join(", ")}`).toEqual([]);
  }
});
