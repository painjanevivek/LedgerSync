import { expect, test } from "@playwright/test";

import { deliveryEvent, mockOperatorConsole, sourceAccount } from "./fixtures";

test("same-account refresh failure retains timestamped historical balance and history", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  await expect(page.getByLabel("INR 1250.00 posted balance")).toBeVisible();
  await page.unroute("**/api/accounts/*/balance");
  await page.route("**/api/accounts/*/balance", (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "temporary_unavailable" } }) }));
  await page.getByRole("button", { name: "Refresh evidence" }).click();
  await expect(page.getByLabel("INR 1250.00 historical posted balance")).toBeVisible();
  await expect(page.getByText(/Balance evidence not refreshed/)).toBeVisible();
  await expect(page.getByText("transfer-existing", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Close account" })).toBeEnabled();
});

test("same-filter event refresh failure retains its last verified page", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/events");
  await expect(page.getByText(deliveryEvent.event_id, { exact: true })).toBeVisible();
  await page.unroute(/\/api\/events\?(?:.*)/);
  await page.route("**/api/events?*", (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "temporary_unavailable" } }) }));
  await page.getByRole("button", { name: "Refresh events" }).click();
  await expect(page.getByText("Event evidence unavailable", { exact: true })).toBeVisible();
  await expect(page.getByText(deliveryEvent.event_id, { exact: true })).toBeVisible();
  await expect(page.getByText(/Event page not refreshed/)).toBeVisible();
});

test("failed account picker is unavailable rather than an unfunded-account business state", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.unroute("**/api/me/accounts?*");
  await page.route("**/api/me/accounts?*", (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "temporary_unavailable" } }) }));
  await page.goto("/transfers");
  await expect(page.getByText("Account picker unavailable", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Retry account picker only" })).toBeVisible();
  await expect(page.getByText("No funded source account", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Review transfer" })).toHaveCount(0);
});

test("overview preserves independent evidence when transfer history is unavailable", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.unroute("**/api/transfers?*");
  await page.route("**/api/transfers?*", (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "temporary_unavailable" } }) }));
  await page.goto("/");
  await expect(page.getByText("Operating-controlled balances", { exact: true })).toBeVisible();
  await expect(page.getByText("Latest reconciliation passed", { exact: true })).toBeVisible();
  await expect(page.getByText("Transfer history unavailable", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Retry transfers only" })).toBeVisible();
  await expect(page.getByText("No transfer records", { exact: true })).toHaveCount(0);
});

test("reconciliation command waits for verified history instead of treating failure as idle", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.unroute("**/api/reconciliation/runs?*");
  await page.route("**/api/reconciliation/runs?*", (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "temporary_unavailable" } }) }));
  await page.goto("/reconciliation");
  await expect(page.getByRole("button", { name: "Run reconciliation", exact: true })).toBeDisabled();
  await expect(page.getByText(/Starting a run is disabled until current reconciliation history is verified/)).toBeVisible();
  await expect(page.getByRole("button", { name: "Retry reconciliation history only" })).toBeVisible();
  await expect(page.getByText("No reconciliation evidence", { exact: true })).toHaveCount(0);
});

test("compact navigation is modal, traps focus, and excludes the background", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockOperatorConsole(page);
  await page.goto("/");
  await page.getByRole("button", { name: /menu/i }).click();
  const drawer = page.getByRole("dialog", { name: "LedgerSync workspace" });
  await expect(drawer).toBeVisible();
  await expect(page.locator("main")).toHaveAttribute("inert", "");
  await expect(drawer.getByRole("button", { name: "Close navigation" })).toBeFocused();
  await page.keyboard.press("Shift+Tab");
  await expect(drawer.getByRole("button", { name: "Sign out" })).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(drawer.getByRole("button", { name: "Close navigation" })).toBeFocused();
});

test("canonical type, touch targets, and reduced motion hold at tablet width", async ({ page }) => {
  await page.setViewportSize({ width: 768, height: 1024 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await mockOperatorConsole(page);
  await page.goto("/transfers");
  await expect(page.getByRole("region", { name: "Transfer comparison" })).toBeVisible();
  const metrics = await page.evaluate(() => {
    const px = (selector: string) => getComputedStyle(document.querySelector(selector)!).fontSize;
    const targetHeights = [...document.querySelectorAll<HTMLElement>("button:not([disabled]), a.button")].filter((element) => element.getClientRects().length > 0).map((element) => element.getBoundingClientRect().height);
    const duration = getComputedStyle(document.querySelector(".side-nav")!).transitionDuration;
    return { h1: px(".page-header h1"), h2: px("h2"), row: px(".data-table td"), label: px(".data-table th"), minimumTarget: Math.min(...targetHeights), duration, overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth };
  });
  expect(metrics).toMatchObject({ h1: "32px", h2: "18px", row: "14px", label: "12px", overflow: false });
  expect(metrics.minimumTarget).toBeGreaterThanOrEqual(44);
  expect(metrics.duration === "0s" || metrics.duration === "1e-05s" || metrics.duration === "0.00001s").toBe(true);
});

test("local evidence reflows at 320 CSS pixels without page-level clipping", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 640 });
  await mockOperatorConsole(page);
  await page.goto("/local-status");
  await expect(page.getByRole("heading", { name: "Local status" })).toBeVisible();
  const reflow = await page.evaluate(() => ({
    pageOverflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
    offenders: [...document.querySelectorAll<HTMLElement>("body *")]
      .filter((element) => !element.closest(".data-table-wrap") && element.getBoundingClientRect().right > document.documentElement.clientWidth + 0.5)
      .map((element) => `${element.tagName.toLowerCase()}.${element.className}:${element.getBoundingClientRect().right.toFixed(2)}`)
      .slice(0, 8),
  }));
  expect(reflow).toEqual({ pageOverflow: 0, offenders: [] });
});
