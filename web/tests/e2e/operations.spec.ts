import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page, type Route } from "@playwright/test";

import { deliveryEvent, diagnostics, eventDetail, mockOperatorConsole, webhookEndpoint, webhookEndpointDetail } from "./fixtures";

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

test("webhook endpoint list exposes safe origin and exact URL-backed filters", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/webhooks");
  await expect(page.getByRole("heading", { name:"Webhook endpoints", exact:true })).toBeVisible();
  await expect(page.getByText(webhookEndpoint.origin, { exact:true })).toBeVisible();
  await expect(page.getByText(/2 recent · 1 dead/i)).toBeVisible();
  await expect(page.getByText(/private\/hooks|credential=|signing_key/i)).toHaveCount(0);
  await page.getByLabel("Endpoint status").selectOption("active");
  await page.getByLabel("Subscribed event").fill("transfer.posted");
  const requestPromise = page.waitForRequest((request) => new URL(request.url()).pathname === "/api/webhook-endpoints" && new URL(request.url()).searchParams.get("status") === "active");
  await page.getByRole("button", { name:"Apply filters" }).click();
  const request = await requestPromise;
  expect(Object.fromEntries(new URL(request.url()).searchParams)).toEqual({ eventType:"transfer.posted",limit:"25",status:"active" });
  await expectNoSeriousA11yViolations(page);
});

test("dead webhook detail links event and financial evidence without implying rollback", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto(`/webhooks/${webhookEndpoint.endpoint_id}`);
  await expect(page.getByRole("heading", { level:1,name:webhookEndpoint.label, exact:true })).toBeVisible();
  await expect(page.getByText("Delivery stopped; the financial record is unchanged")).toBeVisible();
  const related = page.getByRole("navigation", { name:"Related evidence for attempt 2" });
  await expect(related.getByRole("link", { name:"Event" })).toHaveAttribute("href", `/events/${deliveryEvent.event_id}?return_to=%2Fwebhooks%2F${webhookEndpoint.endpoint_id}`);
  await expect(related.getByRole("link", { name:"Transfer" })).toHaveAttribute("href", `/transfers/${webhookEndpointDetail.delivery_attempts[0].transfer_id}?return_to=%2Fwebhooks%2F${webhookEndpoint.endpoint_id}`);
  await expect(page.getByText(/replay resends the existing event only/i)).toBeVisible();
  await expect(page.getByText(/endpoint_url|raw payload|private\/hooks/i)).toHaveCount(0);
  await expectNoSeriousA11yViolations(page);
});

test("webhook replay approval is an exact independently handed-off command", async ({ page }) => {
  await mockOperatorConsole(page);
  const approvalId = "13131313-1313-4313-8313-131313131313";
  let approvalKey = "";
  await page.route(`**/api/webhook-endpoints/${webhookEndpoint.endpoint_id}/deliveries/${webhookEndpointDetail.delivery_attempts[0].attempt_id}/replay-approvals`, async (route) => {
    approvalKey = route.request().headers()["idempotency-key"] ?? "";
    expect(route.request().postDataJSON()).toEqual({ reason_code:"endpoint_restored" });
    expect(route.request().headers()["x-csrf-token"]).toBe("csrf-test-token");
    return json(route, { approval_id:approvalId,status:"approved" }, 201);
  });
  await page.goto(`/webhooks/${webhookEndpoint.endpoint_id}`);
  await page.getByText("Controlled delivery replay").click();
  await page.getByRole("button", { name:"Record replay approval" }).click();
  await expect(page.getByText("Approval recorded for independent handoff")).toBeVisible();
  await expect(page.getByRole("button", { name:"Copy replay approval ID" })).toBeVisible();
  expect(approvalKey).toMatch(/^webhook-approval-/);
  await expect(page.getByText(/cannot edit the payload, destination, transfer, or ledger/i)).toBeVisible();
});

test("unknown webhook replay execution retains the exact approved command", async ({ page }) => {
  await mockOperatorConsole(page);
  const approvalId = "13131313-1313-4313-8313-131313131313";
  let executionKey = "";
  await page.route(`**/api/webhook-endpoints/${webhookEndpoint.endpoint_id}/deliveries/${webhookEndpointDetail.delivery_attempts[0].attempt_id}/replay`, async (route) => {
    executionKey = route.request().headers()["idempotency-key"] ?? "";
    expect(route.request().postDataJSON()).toEqual({ approval_id:approvalId });
    return json(route, { error:{ code:"execution_outcome_unknown" } }, 504);
  });
  await page.goto(`/webhooks/${webhookEndpoint.endpoint_id}`);
  await page.getByText("Controlled delivery replay").click();
  await page.getByLabel("Approved command ID").fill(approvalId);
  await page.getByRole("button", { name:"Schedule existing event replay" }).click();
  await expect(page.getByText("Replay command outcome unknown")).toBeVisible();
  await expect(page.getByRole("button", { name:"Retry exact execution" })).toBeVisible();
  await expect(page.getByLabel("Approved command ID")).toHaveAttribute("readonly", "");
  expect(executionKey).toMatch(/^webhook-execution-/);
});

test("disabled webhook status is explicit and does not erase prior delivered evidence", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route(`**/api/webhook-endpoints/${webhookEndpoint.endpoint_id}`, (route) => json(route, { ...webhookEndpointDetail,status:"disabled",disabled_at:"2026-08-19T12:00:00Z",recent_delivery_state:"delivered",recent_dead_count:"0" }));
  await page.goto(`/webhooks/${webhookEndpoint.endpoint_id}`);
  await expect(page.getByText("Disabled", { exact:true }).first()).toBeVisible();
  await expect(page.getByText("Delivered", { exact:true })).toBeVisible();
  await expect(page.getByText("2026-08-19 12:00:00 UTC", { exact:true })).toBeVisible();
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
  let webhookRequested = false;
  await page.route("**/api/webhook-endpoints?*", (route) => { webhookRequested = true; return json(route, {items:[],next_cursor:""}); });
  await page.goto("/webhooks");
  await expect(page.getByText("Webhook evidence not authorized")).toBeVisible();
  expect(webhookRequested).toBe(false);
});

test("operations screens reflow at 320px and 200-percent-equivalent width with forced colors and reduced motion", async ({ page }) => {
  await mockOperatorConsole(page);
  for (const viewport of [{width:320,height:760},{width:640,height:900},{width:1440,height:900}]) {
    await page.setViewportSize(viewport);
    await page.goto(viewport.width === 640 ? `/events/${deliveryEvent.event_id}` : "/local-status");
    await expect(page.locator("body")).toHaveJSProperty("scrollWidth", viewport.width);
  }
  await page.setViewportSize({width:320,height:760});
  await page.goto(`/webhooks/${webhookEndpoint.endpoint_id}`);
  await expect(page.locator("body")).toHaveJSProperty("scrollWidth", 320);
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
