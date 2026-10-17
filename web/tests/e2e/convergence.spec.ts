import { expect, test } from "@playwright/test";

import { deliveryEvent, mockOperatorConsole, sourceAccount } from "./fixtures";

test("same-account refresh failure retains timestamped historical balance and history", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  await expect(page.getByLabel("INR 1250.00 available balance")).toBeVisible();
  await page.getByText("Transactions and export", { exact: true }).click();
  await page.unroute("**/api/accounts/*/balance");
  await page.route("**/api/accounts/*/balance", (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "temporary_unavailable" } }) }));
  await page.getByRole("button", { name: "Refresh accounts" }).click();
  await expect(page.getByLabel("INR 1250.00 historical available balance")).toBeVisible();
  await expect(page.getByText(/Balance details not refreshed/)).toBeVisible();
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

test("an offline transition during refresh retains the last verified event page", async ({ page, context }) => {
  await mockOperatorConsole(page);
  await page.goto("/events");
  await expect(page.getByText(deliveryEvent.event_id, { exact: true })).toBeVisible();

  let signalRefreshStarted: (() => void) | undefined;
  let releaseRefresh: (() => void) | undefined;
  const refreshStarted = new Promise<void>((resolve) => {
    signalRefreshStarted = resolve;
  });
  const refreshReleased = new Promise<void>((resolve) => {
    releaseRefresh = resolve;
  });

  await page.unroute(/\/api\/events\?(?:.*)/);
  await page.route("**/api/events?*", async (route) => {
    signalRefreshStarted?.();
    await refreshReleased;
    await route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "temporary_unavailable" } }) });
  });

  await page.getByRole("button", { name: "Refresh events" }).click();
  await refreshStarted;
  await context.setOffline(true);
  await page.evaluate(() => window.dispatchEvent(new Event("offline")));

  await expect(page.getByText("You are offline.")).toBeVisible();
  await expect(page.getByText(deliveryEvent.event_id, { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Refresh events" })).toBeDisabled();

  releaseRefresh?.();
  await expect(page.getByText(/Event page not refreshed/)).toBeVisible();
  await expect(page.getByText(deliveryEvent.event_id, { exact: true })).toBeVisible();
  await context.setOffline(false);
});

test("failed account picker is unavailable rather than an unfunded-account business state", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.unroute("**/api/me/accounts?*");
  await page.route("**/api/me/accounts?*", (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "temporary_unavailable" } }) }));
  await page.goto("/transfers/new");
  await expect(page.getByText("Accounts could not be checked", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Retry account check" })).toBeVisible();
  await expect(page.getByText("No funded source account", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Review transfer" })).toHaveCount(0);
});

test("overview preserves independent evidence when transfer history is unavailable", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.unroute("**/api/transfers?*");
  await page.route("**/api/transfers?*", (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "temporary_unavailable" } }) }));
  await page.goto("/");
  await expect(page.getByText("Operating-controlled balances", { exact: true })).toBeVisible();
  await expect(page.getByRole("region", { name: "Reconciliation results state" }).getByText("Internal reconciliation passed", { exact: true })).toBeVisible();
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
  await expect(page.locator(".workspace-column")).toHaveAttribute("inert", "");
  await expect(drawer.getByRole("button", { name: "Close navigation" })).toBeFocused();
  const lastVisibleFocusable = drawer.locator("a[href], button:not([disabled]), summary, [tabindex]:not([tabindex='-1'])").filter({ visible: true }).last();
  await expect(lastVisibleFocusable).toHaveText("Expert tools ");
  await page.keyboard.press("Shift+Tab");
  await expect(lastVisibleFocusable).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(drawer.getByRole("button", { name: "Close navigation" })).toBeFocused();
});

test("canonical type, touch targets, and reduced motion hold at tablet width", async ({ page }) => {
  await page.setViewportSize({ width: 768, height: 1024 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await mockOperatorConsole(page);
  await page.goto("/transfers");
  await expect(page.locator(".transfer-page-heading").getByRole("heading", { name: "Transfers", exact: true })).toBeVisible();
  await expect(page.getByRole("region", { name: "Transfer comparison" })).toBeVisible();
  const metrics = await page.evaluate(() => {
    const px = (selector: string) => {
      const element = document.querySelector(selector);
      if (!element) throw new Error(`Typography source is missing: ${selector}`);
      return Number.parseFloat(getComputedStyle(element).fontSize);
    };
    const targetHeights = [...document.querySelectorAll<HTMLElement>("button:not([disabled]), a.button, input:not([type='hidden']):not([disabled]), select:not([disabled]), summary")].filter((element) => element.checkVisibility({ visibilityProperty: true })).map((element) => element.getBoundingClientRect().height);
    const metadataSizes = [...document.querySelectorAll<HTMLElement>(".eyebrow, .evidence-freshness, .proof-label-row, .folio-field-label, .data-table th, .data-table code, .data-table .status-label")].filter((element) => element.checkVisibility({ visibilityProperty: true })).map((element) => Number.parseFloat(getComputedStyle(element).fontSize));
    const tokens = getComputedStyle(document.documentElement);
    const duration = getComputedStyle(document.querySelector(".guided-topbar")!).transitionDuration;
    return { h1: px(".transfer-page-heading h1"), row: px(".transfer-table td"), label: px(".transfer-table th"), bodyToken: Number.parseFloat(tokens.getPropertyValue("--type-body")), metaToken: Number.parseFloat(tokens.getPropertyValue("--type-meta")), metadataFloor: Math.min(...metadataSizes), minimumTarget: Math.min(...targetHeights), duration, overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth };
  });
  expect(metrics).toMatchObject({ row: 14, label: 12, bodyToken: 14, metaToken: 12, overflow: false });
  expect(metrics.row).toBe(metrics.bodyToken);
  expect(metrics.label).toBe(metrics.metaToken);
  expect(metrics.h1).toBeGreaterThanOrEqual(27);
  expect(metrics.h1).toBeLessThanOrEqual(36);
  expect(metrics.metadataFloor).toBeGreaterThanOrEqual(12);
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
