import { expect, test, type Locator, type Page, type Route } from "@playwright/test";

import { deliveryEvent, destinationAccount, mockOperatorConsole, run, sourceAccount, transfer } from "./fixtures";

const compact = { width: 390, height: 844 };
const tablet = { width: 768, height: 1024 };
const desktop = { width: 1440, height: 900 };
const ultrawide = { width: 2560, height: 1440 };

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({
    status,
    contentType: "application/json",
    headers: { "X-Request-ID": "visual-request-reference" },
    body: JSON.stringify(body),
  });
}

async function capture(page: Page, name: string, viewport = desktop, mask: Locator[] = []) {
  await page.setViewportSize(viewport);
  await page.evaluate(() => document.fonts.ready);
  await expect(page).toHaveScreenshot(`${name}-${viewport.width}x${viewport.height}.png`, {
    animations: "disabled",
    caret: "hide",
    fullPage: true,
    mask,
    maskColor: "#f4f6f8",
    maxDiffPixelRatio: 0.002,
  });
}

const populatedRoutes = [
  { name: "overview-populated", path: "/", heading: "Overview" },
  { name: "accounts-populated", path: "/accounts", heading: "Accounts" },
  { name: "account-detail-populated", path: `/accounts/${sourceAccount.account_id}`, heading: sourceAccount.display_name },
  { name: "funding-records-populated", path: "/funding", heading: "Funding records" },
  { name: "approvals-populated", path: "/approvals", heading: "Approvals" },
  { name: "transfers-populated", path: "/transfers", heading: "Transfers" },
  { name: "transfer-detail-posted-delivery-retrying", path: `/transfers/${transfer.transfer_id}`, heading: "Transfer detail" },
  { name: "reconciliation-populated", path: "/reconciliation", heading: "Reconciliation" },
  { name: "reconciliation-detail-populated", path: `/reconciliation/${run.run_id}`, heading: "Reconciliation details" },
  { name: "local-status-degraded", path: "/local-status", heading: "Local status" },
  { name: "events-populated", path: "/events", heading: "Delivery events" },
  { name: "event-detail-retrying", path: `/events/${deliveryEvent.event_id}`, heading: "Event detail" },
  { name: "developer-contract", path: "/developer", heading: "Developer" },
  { name: "recovery-evidence", path: "/recovery", heading: "Recovery" },
] as const;

for (const route of populatedRoutes) {
  test(`${route.name} has a reviewed desktop baseline`, async ({ page }) => {
    await mockOperatorConsole(page);
    await page.goto(route.path);
    await expect(page.getByRole("heading", { name: route.heading, exact: true })).toBeVisible();
    await capture(page, route.name);
  });
}

test("funding records use the reviewed contextual rail on ultrawide screens", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/funding");
  await expect(page.getByRole("heading", { name: "Funding records", exact: true })).toBeVisible();
  await capture(page, "funding-records-populated-ultrawide", ultrawide);
});

test("funding intake keeps its financial controls readable from desktop to ultrawide", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/funding");
  await page.getByRole("button", { name: "Record funding" }).click();
  await expect(page.getByRole("heading", { name: "Add a funding record", exact: true })).toBeVisible();
  await capture(page, "funding-intake-desktop", desktop);
  await capture(page, "funding-intake-ultrawide", ultrawide);
});

test("compact account directory preserves the selected information hierarchy", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/accounts");
  await expect(page.getByRole("heading", { name: "Accounts", exact: true })).toBeVisible();
  await capture(page, "accounts-populated-compact", compact);
});

test("compact developer contract preserves code and retry hierarchy",async({page})=>{
  await mockOperatorConsole(page);
  await page.goto("/developer");
  await expect(page.getByRole("heading",{name:"Developer",exact:true})).toBeVisible();
  await capture(page,"developer-contract-compact",compact);
});

test("transfer export review preserves the exact evidence hierarchy",async({page})=>{
  await mockOperatorConsole(page);
  await page.goto("/transfers");
  await page.getByRole("button",{name:"Export transfer details"}).click();
  await expect(page.getByRole("heading",{name:"Review transfer history export"})).toBeVisible();
  await capture(page,"transfer-export-review",desktop);
  await capture(page,"transfer-export-review-compact",compact);
});

test("loading state does not imply empty account evidence", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/me/accounts?*", () => new Promise(() => undefined));
  await page.goto("/accounts");
  await expect(page.getByText("Loading authorized accounts")).toBeVisible();
  await capture(page, "accounts-loading", tablet);
});

test("empty account scope is explicit", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/me/accounts?*", (route) => json(route, { accounts: [], next_cursor: "" }));
  await page.goto("/accounts");
  await expect(page.getByText("No accounts yet")).toBeVisible();
  await capture(page, "accounts-empty", compact);
});

test("account dependency failure does not render stale balances", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/me/accounts?*", (route) => json(route, { error: { code: "temporary_unavailable" } }, 503));
  await page.goto("/accounts");
  await expect(page.getByText("Accounts unavailable")).toBeVisible();
  await capture(page, "accounts-error", tablet);
});

test("permission denial remains distinct from an empty directory", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/me/accounts?*", (route) => json(route, { error: { code: "forbidden" } }, 403));
  await page.goto("/accounts");
  await expect(page.getByText("Accounts unavailable")).toBeVisible();
  await capture(page, "accounts-permission-denied", compact);
});

test("offline state preserves already verified evidence and disables writes", async ({ page, context }) => {
  await page.clock.setFixedTime(new Date("2026-08-28T19:23:41Z"));
  await mockOperatorConsole(page);
  await page.goto("/accounts");
  await expect(page.getByRole("heading", { name: "Accounts", exact: true })).toBeVisible();
  await context.setOffline(true);
  await expect(page.getByText("You are offline.")).toBeVisible();
  await capture(page, "accounts-offline", compact, [page.locator(".console-footer")]);
  await context.setOffline(false);
});

test("account detail separates balance failure from history permission", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/accounts/*/balance", (route) => json(route, { error: { code: "temporary_unavailable" } }, 503));
  await page.route("**/api/accounts/*/transactions?*", (route) => json(route, { error: { code: "forbidden" } }, 403));
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  await expect(page.getByText("Temporarily unavailable")).toBeVisible();
  await expect(page.getByText("Your role is not authorized to view ledger history.")).toBeVisible();
  await capture(page, "account-detail-independent-failures", desktop);
});

test("unknown transfer outcome keeps the exact intent and safe retry action", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/me/accounts?*", (route) => json(route, { accounts: [sourceAccount, destinationAccount], next_cursor: "" }));
  await page.route("**/api/transfers", (route) => route.request().method() === "POST" ? json(route, { error: { code: "transfer_outcome_unknown" } }, 504) : route.fallback());
  await page.goto("/transfers");
  await expect(page.getByRole("heading", { name: "Internal transfer" })).toBeVisible();
  await page.getByLabel("Amount").fill("12.50");
  await page.getByRole("button", { name: "Review transfer" }).click();
  await page.getByRole("button", { name: "Confirm and post" }).click();
  await expect(page.getByText("Result not yet confirmed")).toBeVisible();
  await capture(page, "transfer-unknown-outcome", compact);
});

test("reconciliation mismatch is visually stop-ship", async ({ page }) => {
  const mismatch = { ...run, status: "mismatch", mismatch_count: "2" };
  await mockOperatorConsole(page);
  await page.route("**/api/reconciliation/runs?*", (route) => json(route, { runs: [mismatch], next_cursor: "" }));
  await page.goto("/reconciliation");
  await expect(page.getByText("Mismatch detected", { exact: true }).first()).toBeVisible();
  await capture(page, "reconciliation-mismatch", compact);
});

test("missing session renders the login layer with no financial evidence", async ({ page }) => {
  await page.route("**/api/session", (route) => json(route, { error: { code: "unauthorized" } }, 401));
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Your ledger starts empty." })).toBeVisible();
  await capture(page, "shell-permission-denied", tablet);
});

test("read-only transfer role explains why posting is disabled", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/session", (route) => json(route, { subject_id: "auditor-1", tenant_id: "tenant-1", csrf_token: "csrf-test-token", scopes: ["accounts:read", "transfers:read"], environment: "local", tenant_label: "My Ledger Workspace", operator_label: "Read-only auditor" }));
  await page.goto("/transfers");
  await expect(
    page.getByRole("region", { name: "Internal transfer" }).locator("#transfer-disabled-reason"),
  ).toHaveText("Read-only role: transfer posting is not permitted.");
  await capture(page, "transfers-read-only-capability", desktop);
});

test("mixed-currency overview refuses a false aggregate", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/me/accounts?*", (route) => json(route, { accounts: [sourceAccount, { ...destinationAccount, currency: "EUR" }], next_cursor: "" }));
  await page.goto("/");
  await expect(page.getByText("Mixed-currency pilot data blocked")).toBeVisible();
  await capture(page, "overview-mixed-currency-error", desktop);
});

test("account create review has a Windows local-console baseline", async ({ page }) => {
  test.skip(process.platform !== "win32", "The supported local product environment is the reviewed Windows workstation.");
  await mockOperatorConsole(page);
  await page.goto("/accounts/new");
  await page.getByLabel("Display name").fill("Settlement evidence");
  await page.getByLabel("External reference").fill("SETTLEMENT-INR");
  await page.getByLabel("Category").selectOption("operating");
  await page.getByRole("button", { name: "Continue to financial boundary" }).click();
  await page.getByRole("button", { name: "Continue to review" }).click();
  await expect(page.getByRole("heading", { name: "Review exact account command" })).toBeVisible();
  await capture(page, "account-create-review", desktop);
  await capture(page, "account-create-review-compact", compact);
});

test("account lifecycle guard has a Windows local-console baseline", async ({ page }) => {
  test.skip(process.platform !== "win32", "The supported local product environment is the reviewed Windows workstation.");
  await mockOperatorConsole(page);
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  await page.getByRole("button", { name: "Freeze account" }).click();
  await page.getByLabel("Reason").fill("Temporary review of duplicate instructions");
  await expect(page.getByRole("heading", { name: "Freeze account" })).toBeVisible();
  await page.setViewportSize(desktop);
  await page.evaluate(() => document.fonts.ready);
  await expect(page).toHaveScreenshot("account-freeze-confirmation-1440x900.png", { animations: "disabled", caret: "hide", fullPage: false, maxDiffPixelRatio: 0.002 });
  await page.keyboard.press("Escape");
  await page.setViewportSize(compact);
  await page.getByRole("button", { name: "Freeze account" }).click();
  await page.getByLabel("Reason").fill("Temporary review of duplicate instructions");
  await expect(page.getByRole("heading", { name: "Freeze account" })).toBeVisible();
  await page.evaluate(() => document.fonts.ready);
  await expect(page).toHaveScreenshot("account-freeze-confirmation-compact-390x844.png", { animations: "disabled", caret: "hide", fullPage: false, maxDiffPixelRatio: 0.002 });
});

test("reconciliation command review has a Windows local-console baseline", async ({ page }) => {
  test.skip(process.platform !== "win32", "The supported local product environment is the reviewed Windows workstation.");
  await mockOperatorConsole(page);
  await page.goto("/reconciliation");
  await page.getByRole("button", { name: "Run reconciliation", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Review authoritative reconciliation scope" })).toBeVisible();
  await capture(page, "reconciliation-command-review", desktop);
  await capture(page, "reconciliation-command-review-compact", compact);
});

test("reconciliation running control has a Windows local-console baseline", async ({ page }) => {
  test.skip(process.platform !== "win32", "The supported local product environment is the reviewed Windows workstation.");
  const running = { ...run, run_id: "77777777-7777-4777-8777-777777777777", status: "running", ledger_watermark: "", application_version: "", schema_version: "", checked_account_count: "0", posting_count: "0", mismatch_count: "0", completed_at: "" };
  await mockOperatorConsole(page);
  await page.route("**/api/reconciliation/runs", (route) => json(route, running, 202));
  await page.route(`**/api/reconciliation/runs/${running.run_id}`, (route) => json(route, running));
  await page.goto("/reconciliation");
  await page.getByRole("button", { name: "Run reconciliation", exact: true }).click();
  await page.getByRole("button", { name: "Start reconciliation" }).click();
  await expect(page.getByRole("heading", { name: "Reconciliation running" })).toBeVisible();
  await capture(page, "reconciliation-command-running", desktop);
  await capture(page, "reconciliation-command-running-compact", compact);
});
