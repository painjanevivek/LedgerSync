import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

import { mockOperatorConsole, run, sourceAccount, transfer } from "./fixtures";

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
  // Chromium exposes forced system colors incompletely to axe. Preserve the
  // forced-color interaction check above, then run the complete contrast rule
  // set in the deterministic authored-color mode rather than disabling it.
  await page.emulateMedia({ colorScheme:"light", reducedMotion:"reduce", forcedColors:"none" });
  const results=await new AxeBuilder({page}).analyze(); expect(results.violations).toEqual([]);
});

test("320 CSS-pixel reflow, equivalent to 400% desktop zoom, keeps evidence available", async ({ page }) => {
  await page.setViewportSize({width:320,height:640}); await mockOperatorConsole(page); await page.goto("/transfers");
  await expect(page.getByRole("heading",{name:"Transfers",exact:true})).toBeVisible();
  const reflow = await page.evaluate(() => {
    const root = document.documentElement;
    const offenders = [...document.querySelectorAll<HTMLElement>("body *")]
      .filter((element) => element.getBoundingClientRect().right > root.clientWidth + 1)
      .map((element) => `${element.tagName.toLowerCase()}.${element.className}:${Math.round(element.getBoundingClientRect().right)}`)
      .slice(0, 8);
    return { horizontalOverflow: root.scrollWidth > root.clientWidth, overflowPixels: root.scrollWidth - root.clientWidth, offenders };
  });
  expect(reflow).toEqual({ horizontalOverflow: false, overflowPixels: 0, offenders: [] });
});

test("copy controls announce completion without hiding the full identifier", async ({page})=>{
  await page.context().grantPermissions(["clipboard-read","clipboard-write"]); await mockOperatorConsole(page); await page.goto("/reconciliation");
  await page.getByRole("button",{name:"Copy identifier"}).first().click(); await expect(page.getByText("Copied",{exact:true}).first()).toBeAttached();
});

test("account balance and ledger history report independent truth states", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.unroute("**/api/accounts/*/transactions?*");
  await page.route("**/api/accounts/*/transactions?*", (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "history_unavailable" } }) }));
  await page.goto(`/accounts/11111111-1111-4111-8111-111111111111`);
  await expect(page.getByText("USD 1250.00")).toBeVisible();
  await expect(page.getByText("Ledger history unavailable", { exact: true })).toBeVisible();
  await expect(page.getByText("No ledger entries")).toHaveCount(0);
});

test("every populated MVP route has no automatically detectable WCAG A or AA violation", async ({ page }) => {
  await mockOperatorConsole(page);
  const routes = ["/accounts", `/accounts/${sourceAccount.account_id}`, "/transfers", `/transfers/${transfer.transfer_id}`, "/reconciliation", `/reconciliation/${run.run_id}`];
  for (const route of routes) {
    await page.goto(route);
    await expect(page.locator("main h1")).toBeVisible();
    const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
    expect(results.violations, `${route}: ${results.violations.map((violation) => violation.id).join(", ")}`).toEqual([]);
  }
});

test("keyboard-only transfer review announces an unknown result and preserves the retry key", async ({ page }) => {
  await mockOperatorConsole(page);
  const keys: string[] = [];
  await page.route("**/api/transfers", (route) => {
    if (route.request().method() !== "POST") return route.fallback();
    keys.push(route.request().headers()["idempotency-key"] ?? "");
    return route.abort("failed");
  });
  await page.goto("/transfers");
  const amount = page.getByLabel("Exact amount");
  await amount.focus();
  await amount.fill("12.50");
  const review = page.getByRole("button", { name: "Review transfer" });
  await review.focus();
  await review.press("Enter");
  await expect(page.getByRole("heading", { name: "Confirm exact transfer" })).toBeFocused();
  const confirm = page.getByRole("button", { name: "Confirm and post" });
  await confirm.focus();
  await confirm.press("Enter");
  await expect(page.getByRole("status").filter({ hasText: "Result not yet confirmed" })).toBeVisible();
  const retry = page.getByRole("button", { name: "Retry same transfer" });
  await retry.focus();
  await retry.press("Enter");
  expect(keys).toHaveLength(2);
  expect(keys[0]).toBeTruthy();
  expect(keys[1]).toBe(keys[0]);
});

test("exact-money input survives phone rotation and retains maximum signed-64-bit evidence", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockOperatorConsole(page);
  await page.goto("/transfers");
  const amount = page.getByLabel("Exact amount");
  await expect(amount).toHaveAttribute("inputmode", "decimal");
  await amount.fill("92233720368547758.07");
  await page.setViewportSize({ width: 844, height: 390 });
  await expect(amount).toHaveValue("92233720368547758.07");
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole("button", { name: "Review transfer" }).click();
  await expect(page.getByText("USD 92233720368547758.07")).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
});

test("WCAG text-spacing overrides preserve compact evidence and controls", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 640 });
  await mockOperatorConsole(page);
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  await page.evaluate((css) => {
    const style = document.createElement("style");
    const nonce = document.querySelector<HTMLScriptElement>("script[nonce]")?.nonce;
    if (nonce) style.nonce = nonce;
    style.textContent = css;
    document.head.append(style);
  }, "* { line-height: 1.5 !important; letter-spacing: .12em !important; word-spacing: .16em !important; } p { margin-bottom: 2em !important; }");
  await expect(page.getByText("USD 1250.00")).toBeVisible();
  await expect(page.getByRole("link", { name: /Back to account directory/ })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
});

test("compact primary actions and navigation meet the 44 CSS-pixel touch target", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockOperatorConsole(page);
  await page.goto("/transfers");
  const targets = [page.getByRole("button", { name: /menu/i }), page.getByRole("button", { name: "Review transfer" }), page.getByRole("button", { name: "Refresh history" })];
  for (const target of targets) {
    const box = await target.boundingBox();
    expect(box, `missing touch target for ${await target.getAttribute("aria-label") ?? await target.textContent()}`).not.toBeNull();
    expect(box!.height).toBeGreaterThanOrEqual(44);
    expect(box!.width).toBeGreaterThanOrEqual(44);
  }
});
