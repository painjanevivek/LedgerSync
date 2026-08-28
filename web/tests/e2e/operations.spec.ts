import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page, type Route } from "@playwright/test";

import { deliveryEvent, diagnostics, eventDetail, mockOperatorConsole } from "./fixtures";

function json(route: Route, body: unknown, status = 200) { return route.fulfill({ status, contentType:"application/json", body:JSON.stringify(body) }); }

async function expectNoSeriousA11yViolations(page: Page) {
  const result = await new AxeBuilder({ page }).exclude(".app-shell > aside").analyze();
  expect(result.violations.filter((violation) => ["serious","critical"].includes(violation.impact ?? ""))).toEqual([]);
}

test("local status orders financial authority, delivery, and disposable cache without conflating them", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/local-status");
  await expect(page.getByRole("heading", { name:"Local status", exact:true })).toBeVisible();
  const ledger = page.getByRole("region", { name:"Ordered local truth domains" });
  await expect(ledger.getByRole("heading", { name:"PostgreSQL ledger" })).toBeVisible();
  await expect(ledger.getByRole("heading", { name:"Transactional outbox" })).toBeVisible();
  await expect(ledger.getByRole("heading", { name:"Redis cache" })).toBeVisible();
  await expect(ledger.getByText(/cache outage may slow reads.*never evidence that PostgreSQL money changed/i)).toBeVisible();
  await expect(page.getByRole("link", { name:"Investigate delivery events" })).toHaveAttribute("href", "/events");
  await expect(page.getByRole("button", { name:"Copy local status command" })).toBeVisible();
  await expectNoSeriousA11yViolations(page);
});

test("partial dependency failure remains a truthful partial snapshot", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/local/diagnostics", (route) => json(route, { ...diagnostics,overall_state:"unavailable",financial_authority:{postgres:{state:"unavailable"},latest_reconciliation:{state:"unavailable"}},delivery_cache:{...diagnostics.delivery_cache,outbox:{state:"unavailable",worker_progress:"unknown"}} }));
  await page.goto("/local-status");
  await expect(page.getByRole("heading", { name:"Unavailable", exact:true })).toBeVisible();
  await expect(page.getByText("Not available", { exact:true }).first()).toBeVisible();
  await expect(page.getByText(/financial results remain unknown unless separately verified/i)).toBeVisible();
});

test("event filters use the exact allowlisted query and an empty result is explicit", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/events?*", (route) => json(route, { events:[],next_cursor:"" }));
  await page.goto("/events");
  await page.getByLabel("Event type").fill("account.balance.changed.v1");
  await page.getByLabel("State").selectOption("dead");
  await page.getByLabel("Related ID").fill(deliveryEvent.aggregate_id);
  await page.getByLabel("From UTC").fill("2026-08-19T00:00:00Z");
  const filteredRequestPromise = page.waitForRequest((request) => {
    const url = new URL(request.url());
    return url.pathname === "/api/events" && url.searchParams.has("eventType");
  });
  await page.getByRole("button", { name:"Apply filters" }).click();
  const filteredRequest = await filteredRequestPromise;
  await expect(page.getByText("No events match these filters")).toBeVisible();
  expect(Object.fromEntries(new URL(filteredRequest.url()).searchParams)).toEqual({
    eventType: "account.balance.changed.v1",
    from: "2026-08-19T00:00:00Z",
    limit: "25",
    relatedId: deliveryEvent.aggregate_id,
    state: "dead",
  });
});

test("dead event detail is read-only and sends the operator to authoritative related evidence", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route(`**/api/events/${deliveryEvent.event_id}`, (route) => json(route, { ...eventDetail,state:"dead",dead_at:"2026-08-19T11:02:00Z",delivery_attempts_truncated:true }));
  await page.goto(`/events/${deliveryEvent.event_id}`);
  await expect(page.getByRole("heading", { name:"Event detail", exact:true })).toBeVisible();
  await expect(page.getByText("Delivery stopped; financial status is not inferred")).toBeVisible();
  await expect(page.getByText(/transfer may already be posted in PostgreSQL/i)).toBeVisible();
  await expect(page.getByRole("link", { name:"Open transfer evidence" })).toHaveAttribute("href", `/transfers/${deliveryEvent.transfer_id}`);
  await expect(page.getByRole("link", { name:"Open account evidence" })).toHaveAttribute("href", `/accounts/${deliveryEvent.account_id}`);
  await expect(page.getByText("Older attempts are not shown")).toBeVisible();
  await expect(page.getByRole("button", { name:/replay/i })).toHaveCount(0);
  await expectNoSeriousA11yViolations(page);
});

test("event list exposes authorized related evidence links", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/events");
  const related = page.getByRole("navigation", { name:`Related evidence for ${deliveryEvent.event_id}` });
  await expect(related.getByRole("link", { name:"Transfer" })).toHaveAttribute("href", `/transfers/${deliveryEvent.transfer_id}?return_to=%2Fevents`);
  await expect(related.getByRole("link", { name:"Account" })).toHaveAttribute("href", `/accounts/${deliveryEvent.account_id}?return_to=%2Fevents`);
});

test("missing operations scopes are distinct from empty evidence and do not call the BFF", async ({ page }) => {
  await mockOperatorConsole(page);
  let requested = false;
  await page.route("**/api/session", (route) => json(route, { subject_id:"auditor",tenant_id:"tenant-1",csrf_token:"csrf",scopes:[],environment:"local",tenant_label:"My Ledger Workspace",operator_label:"Read-only auditor" }));
  await page.route("**/api/local/diagnostics", (route) => { requested = true; return json(route, diagnostics); });
  await page.goto("/local-status");
  await expect(page.getByText("Local diagnostics not authorized")).toBeVisible();
  expect(requested).toBe(false);
  await page.goto("/events");
  await expect(page.getByText("Event evidence not authorized")).toBeVisible();
});

test("operations screens reflow at 320px and 200-percent-equivalent width with forced colors and reduced motion", async ({ page }) => {
  await mockOperatorConsole(page);
  for (const viewport of [{width:320,height:760},{width:640,height:900},{width:1440,height:900}]) {
    await page.setViewportSize(viewport);
    await page.goto(viewport.width === 640 ? `/events/${deliveryEvent.event_id}` : "/local-status");
    await expect(page.locator("body")).toHaveJSProperty("scrollWidth", viewport.width);
  }
  await page.emulateMedia({ forcedColors:"active", reducedMotion:"reduce" });
  await page.goto(`/events/${deliveryEvent.event_id}`);
  await expect(page.getByText("Delivery is not confirmed; financial status is separate")).toBeVisible();
  await expectNoSeriousA11yViolations(page);
});

test("offline status retains its timestamp but disables refresh", async ({ page, context }) => {
  await mockOperatorConsole(page);
  await page.goto("/local-status");
  await expect(page.getByText("2026-08-19 12:00:00 UTC")).toBeVisible();
  await context.setOffline(true);
  await expect(page.getByText("Offline — evidence is not current")).toBeVisible();
  await expect(page.getByRole("button", { name:"Refresh evidence" })).toBeDisabled();
  await context.setOffline(false);
});
