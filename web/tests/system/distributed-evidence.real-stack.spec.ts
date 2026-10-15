import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { setIsolatedExperienceMode, requireIsolatedRealStack } from "./real-stack";

test("distributed evidence follows the configured feature boundary", async ({ page }, testInfo) => {
  requireIsolatedRealStack();
  await page.goto("/sign-in");
  await page.getByRole("link", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("heading", { name: /^(Overview|Your money at a glance)$/ })).toBeVisible();
  await setIsolatedExperienceMode(page, "simple");
  await page.goto("/developer");
  await expect(page.getByRole("heading", { name: "Developer", exact: true })).toBeVisible();
  const session = await (await page.request.get("/api/session")).json();
  if (session.features?.systems_lab !== true) {
    await expect(page.locator("#developer-distributed-systems")).toHaveCount(0);
    expect((await page.request.get("/api/developer/distributed")).status()).toBe(404);
    expect((await page.request.get("/api/developer/distributed/evidence")).status()).toBe(404);
    const accessibility = await new AxeBuilder({ page }).analyze();
    expect(accessibility.violations).toEqual([]);
    await page.screenshot({ path: testInfo.outputPath("developer-core.png"), fullPage: true });
    return;
  }
  if (process.env.LEDGERSYNC_STUDY_CAPTURE_BASELINE === "true") {
    await page.screenshot({ path: testInfo.outputPath("ui-developer-before.png"), fullPage: true });
    return;
  }
  const disclosure = page.locator("#developer-distributed-systems");
  await expect(disclosure).not.toHaveAttribute("open");
  const summary = disclosure.locator("summary");
  const observationResponse = page.waitForResponse((response) => response.url().endsWith("/api/developer/distributed"));
  await summary.focus();
  await summary.press("Enter");
  const observation = await observationResponse;
  expect(observation.status()).toBe(200);
  const data = await observation.json();
  expect(data.api_replicas).toMatchObject({ state: "available", count: "2" });
  expect(data.worker_publications.state).toBe("historical");
  expect(data.scheduler.state).toBe("active");
  await expect(disclosure.getByText("Ready API replicas", { exact: true })).toBeVisible();
  await expect(disclosure.getByText("Historical publication evidence", { exact: true })).toBeVisible();
  await expect(disclosure.getByText("Loading observations…", { exact: true })).not.toBeVisible();
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 });
    await expect(summary).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    const accessibility = await new AxeBuilder({ page }).analyze();
    expect(accessibility.violations).toEqual([]);
    await page.screenshot({ path: testInfo.outputPath(`ui-developer-${width}.png`), fullPage: true });
    await page.screenshot({ path: testInfo.outputPath(`ui-developer-${width}-viewport.png`) });
  }
  const evidence = await page.request.get("/api/developer/distributed/evidence");
  await expect(disclosure.getByText("Historical lease — refresh needed", { exact: true })).toBeVisible({ timeout: 6_000 });
  expect(evidence.status()).toBe(200);
  expect(await evidence.text()).toContain("historical study evidence");
  const events = await (await page.request.get("/api/events?limit=1")).json();
  await page.goto(`/events/${events.events[0].event_id}`);
  await page.locator("#event-order summary").click();
  await expect(page.getByRole("heading", { name: "Receive clocks", exact: true })).toBeVisible();
  const runs = await (await page.request.get("/api/reconciliation/runs?limit=1")).json();
  await page.goto(`/reconciliation/${runs.runs[0].run_id}`);
  await page.locator("#reconciliation-snapshot-boundary summary").click();
  await expect(page.getByText("Database snapshot time", { exact: true })).toBeVisible();
  await expect(page.getByText("Repeatable read", { exact: true })).toBeVisible();
});
