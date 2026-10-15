import { expect, test } from "@playwright/test";
import { mockOperatorConsole } from "./fixtures";

test("Systems Lab displays saved synthetic proof and has no browser fault command", async ({ page }) => {
  await mockOperatorConsole(page);
  const writes: string[] = [];
  page.on("request", request => { if (!["GET", "HEAD"].includes(request.method())) writes.push(new URL(request.url()).pathname); });
  await page.goto("/developer");
  await page.locator("#developer-distributed-systems summary").click();
  await expect(page.getByRole("heading", { name: "Last local failure demonstration" })).toBeVisible();
  await expect(page.getByText("All named demonstration checks passed", { exact: true })).toBeVisible();
  await expect(page.getByRole("region", { name: "Local failure demonstration checks" })).toBeVisible();
  await expect(page.getByRole("button", { name: /run.*(demo|fault)|start.*fault/i })).toHaveCount(0);
  expect(writes).toEqual([]);
});
