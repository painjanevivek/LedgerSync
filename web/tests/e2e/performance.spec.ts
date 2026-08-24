import { expect, test, type Page } from "@playwright/test";

import { mockOperatorConsole, sourceAccount } from "./fixtures";

type BrowserMetrics = {
  lcpMilliseconds: number;
  cls: number;
  inpMilliseconds: number;
  interactionCount: number;
  longTaskCount: number;
  longTaskTotalMilliseconds: number;
  maxLongTaskMilliseconds: number;
  shiftSources: string[];
};

async function installWebVitalObservers(page: Page) {
  await page.addInitScript(() => {
    const state = {
      lcpMilliseconds: 0,
      cls: 0,
      inpMilliseconds: 0,
      interactionCount: 0,
      longTaskCount: 0,
      longTaskTotalMilliseconds: 0,
      maxLongTaskMilliseconds: 0,
      shiftSources: [] as string[],
    };
    Object.defineProperty(globalThis, "__ledgerSyncPerformance", { value: state, configurable: true });
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) state.lcpMilliseconds = Math.max(state.lcpMilliseconds, entry.startTime);
    }).observe({ type: "largest-contentful-paint", buffered: true });
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries() as Array<PerformanceEntry & { hadRecentInput?: boolean; value?: number; sources?: Array<{ node?: Node }> }>) {
        if (!entry.hadRecentInput) {
          state.cls += entry.value ?? 0;
          for (const source of entry.sources ?? []) {
            if (state.shiftSources.length < 12 && source.node instanceof Element) state.shiftSources.push(source.node.className || source.node.tagName);
          }
        }
      }
    }).observe({ type: "layout-shift", buffered: true });
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries() as Array<PerformanceEntry & { duration: number; interactionId?: number }>) {
        if ((entry.interactionId ?? 0) > 0) {
          state.interactionCount += 1;
          state.inpMilliseconds = Math.max(state.inpMilliseconds, entry.duration);
        }
      }
    }).observe({ type: "event", buffered: true, durationThreshold: 16 } as PerformanceObserverInit & { durationThreshold: number });
    if (PerformanceObserver.supportedEntryTypes.includes("longtask")) {
      new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          state.longTaskCount += 1;
          state.longTaskTotalMilliseconds += entry.duration;
          state.maxLongTaskMilliseconds = Math.max(state.maxLongTaskMilliseconds, entry.duration);
        }
      }).observe({ type: "longtask", buffered: true });
    }
  });
}

async function metrics(page: Page): Promise<BrowserMetrics> {
  return page.evaluate(() => {
    const state = (globalThis as typeof globalThis & { __ledgerSyncPerformance: BrowserMetrics }).__ledgerSyncPerformance;
    return { ...state };
  });
}

test("compact throttled overview stays inside the pilot web-vital budgets", { tag: "@performance" }, async ({ page, context }, testInfo) => {
  test.setTimeout(60_000);
  await page.setViewportSize({ width: 390, height: 844 });
  const session = await context.newCDPSession(page);
  await session.send("Network.enable");
  await session.send("Network.emulateNetworkConditions", {
    offline: false,
    latency: 75,
    downloadThroughput: (4_000_000 / 8),
    uploadThroughput: (1_500_000 / 8),
    connectionType: "cellular4g",
  });
  await session.send("Emulation.setCPUThrottlingRate", { rate: 4 });
  await installWebVitalObservers(page);
  await mockOperatorConsole(page);
  const boundedRequests: string[] = [];
  const apiRequests: string[] = [];
  page.on("request", (request) => {
    if (!["document", "script", "stylesheet", "fetch", "xhr"].includes(request.resourceType())) return;
    const url = new URL(request.url());
    boundedRequests.push(`${request.method()} ${url.pathname}`);
    if (url.pathname.startsWith("/api/")) apiRequests.push(`${request.method()} ${url.pathname}`);
  });

  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
  await page.waitForTimeout(500);
  const initialLoad = await metrics(page);
  const initialRequestCount = boundedRequests.length;
  const initialApiRequestCount = apiRequests.length;
  console.log(`compact_initial_vitals=${JSON.stringify(initialLoad)}`);
  await page.getByRole("button", { name: "Menu" }).click();
  await page.getByRole("link", { name: "Accounts" }).click();
  await expect(page.getByRole("heading", { name: "Accounts", exact: true })).toBeVisible();
  await page.waitForTimeout(500);

  const observed = await metrics(page);
  const requestFrequency = Object.fromEntries([...new Set(apiRequests)].map((path) => [path, apiRequests.filter((item) => item === path).length]));
  console.log(`compact_observed_vitals=${JSON.stringify(observed)}`);
  console.log(`compact_request_budget=${JSON.stringify({ initialRequestCount, initialApiRequestCount, totalRequestCount: boundedRequests.length, apiRequestCount: apiRequests.length, requestFrequency })}`);
  await testInfo.attach("compact-web-vitals.json", {
    body: Buffer.from(JSON.stringify({
      profile: { viewport: "390x844", cpuThrottle: 4, network: "constrained-4g" },
      initialLoad,
      observed,
      requests: { initialRequestCount, initialApiRequestCount, totalRequestCount: boundedRequests.length, apiRequestCount: apiRequests.length, requestFrequency },
    }, null, 2)),
    contentType: "application/json",
  });
  expect(initialLoad.lcpMilliseconds, "LCP evidence must be present").toBeGreaterThan(0);
  expect(observed.interactionCount, "INP evidence must include a real interaction").toBeGreaterThan(0);
  expect(initialLoad.lcpMilliseconds).toBeLessThanOrEqual(2_500);
  expect(observed.inpMilliseconds).toBeLessThanOrEqual(200);
  expect(initialLoad.cls).toBeLessThanOrEqual(0.1);
  expect(initialRequestCount).toBeLessThanOrEqual(32);
  expect(initialApiRequestCount).toBeLessThanOrEqual(8);
  expect(apiRequests.length).toBeLessThanOrEqual(12);
  expect(Math.max(0, ...Object.values(requestFrequency))).toBeLessThanOrEqual(2);
  expect(initialLoad.maxLongTaskMilliseconds).toBeLessThanOrEqual(250);
  expect(observed.longTaskTotalMilliseconds).toBeLessThanOrEqual(1_500);
});

test("large bounded history progressively renders without blocking navigation", { tag: "@performance" }, async ({ page }) => {
  await mockOperatorConsole(page);
  const transactions = Array.from({ length: 100 }, (_, index) => ({
    transfer_id: `70000000-0000-4000-8000-${String(index).padStart(12, "0")}`,
    direction: index % 2 === 0 ? "credit" : "debit",
    amount: String(100 + index),
    currency: "INR",
    status: "posted",
    occurred_at: `2026-08-19T11:${String(index % 60).padStart(2, "0")}:00Z`,
  }));
  await page.route("**/api/accounts/*/transactions?*", async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 250));
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ transactions, next_cursor: "next-page" }) });
  });

  await page.goto(`/accounts/${sourceAccount.account_id}`);
  await expect(page.getByText("Loading ledger history")).toBeVisible();
  await expect(page.locator(".ledger-row")).toHaveCount(100);
  await expect(page.getByRole("link", { name: "Transfers" })).toBeVisible();
  await page.getByRole("link", { name: "Transfers" }).click();
  await expect(page.getByRole("heading", { name: "Transfers", exact: true })).toBeVisible();
});
