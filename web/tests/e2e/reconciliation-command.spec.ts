import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page, type Route } from "@playwright/test";

import { mockOperatorConsole, run, sourceAccount } from "./fixtures";

function json(route: Route, body: unknown, status = 200, headers: Record<string, string> = {}) {
  return route.fulfill({ status, contentType: "application/json", headers, body: JSON.stringify(body) });
}

const runningRun = {
  ...run,
  run_id: "77777777-7777-4777-8777-777777777777",
  correlation_id: "88888888-8888-4888-8888-888888888888",
  status: "running",
  ledger_watermark: "",
  application_version: "",
  schema_version: "",
  checked_account_count: "0",
  posting_count: "0",
  mismatch_count: "0",
  started_at: "2026-08-25T12:00:00Z",
  completed_at: "",
} as const;

const mismatchRun = {
  ...run,
  run_id: "99999999-9999-4999-8999-999999999999",
  correlation_id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
  status: "mismatch",
  ledger_watermark: "9",
  mismatch_count: "1",
  started_at: "2026-08-25T12:00:00Z",
  completed_at: "2026-08-25T12:00:02Z",
  mismatches: [{
    mismatch_id: "mismatch-1",
    account_id: sourceAccount.account_id,
    classification: "ledger_balance_mismatch",
    currency: "INR",
    expected_minor: "125000",
    observed_minor: "124999",
    observed_available_minor: "124999",
    balance_version: "9",
    created_at: "2026-08-25T12:00:02Z",
  }],
} as const;

async function openReview(page: Page) {
  await page.goto("/reconciliation");
  await page.getByRole("button", { name: "Run reconciliation", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Review authoritative reconciliation scope" })).toBeFocused();
}

test("operator reviews exact scope and a completed run preserves prior history", async ({ page }) => {
  await mockOperatorConsole(page);
  const completed = { ...run, run_id: "abababab-abab-4bab-8bab-abababababab", correlation_id: "cdcdcdcd-cdcd-4dcd-8dcd-cdcdcdcdcdcd", ledger_watermark: "10", completed_at: "2026-08-25T12:00:02Z" };
  let postedKey = "";
  let posted = false;
  await page.route("**/api/reconciliation/runs", (route) => {
    posted = true;
    postedKey = route.request().headers()["idempotency-key"] ?? "";
    expect(route.request().headers()["x-csrf-token"]).toBe("csrf-test-token");
    expect(route.request().postData()).toBe("{}");
    return json(route, completed, 202, { "X-Request-ID": "reconcile-request-1" });
  });
  await page.route("**/api/reconciliation/runs?*", (route) => json(route, { runs: posted ? [completed, run] : [run], next_cursor: "" }));

  await openReview(page);
  const review = page.getByRole("region", { name: "Review authoritative reconciliation scope" });
  await expect(review.getByText("All authorized INR accounts")).toBeVisible();
  await expect(review.getByText("8", { exact: true })).toBeVisible();
  await expect(review.getByText(/no balance, transfer, posting, or journal entry is changed/i)).toBeVisible();
  await review.getByRole("button", { name: "Start reconciliation" }).click();

  await expect(page.getByRole("heading", { name: "Reconciliation passed" })).toBeFocused();
  expect(postedKey).toMatch(/^[\x21-\x7e]{16,255}$/);
  await expect(page.getByText("reconcile-request-1", { exact: true })).toBeVisible();
  const history = page.getByRole("region", { name: "Reconciliation run history" });
  await expect(history.getByText(completed.run_id, { exact: true })).toBeVisible();
  await expect(history.getByText(run.run_id, { exact: true })).toBeVisible();
});

test("running state never infers a result and manual refresh resolves one stable run ID", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/reconciliation/runs", (route) => json(route, runningRun, 202));
  await page.route(`**/api/reconciliation/runs/${runningRun.run_id}`, (route) => json(route, mismatchRun));
  await page.route("**/api/reconciliation/runs?*", (route) => json(route, { runs: [mismatchRun, run], next_cursor: "" }));

  await openReview(page);
  await page.getByRole("button", { name: "Start reconciliation" }).click();
  const running = page.getByRole("status").filter({ has: page.getByRole("heading", { name: "Reconciliation running" }) });
  await expect(running).toBeVisible();
  await expect(running.getByText(/no passing or mismatch result is inferred/i)).toBeVisible();
  await expect(running.getByText(runningRun.run_id, { exact: true })).toBeVisible();
  await running.getByRole("button", { name: "Refresh run status" }).click();
  await expect(page.getByRole("heading", { name: "Mismatch detected", exact: true }).first()).toBeFocused();
  await expect(page.getByRole("link", { name: "Open authoritative run evidence" })).toHaveAttribute("href", `/reconciliation/${mismatchRun.run_id}`);
});

test("response unknown survives reload and retries the exact same key and body", async ({ page }) => {
  await mockOperatorConsole(page);
  const requests: Array<{ key?: string; body: string | null }> = [];
  await page.route("**/api/reconciliation/runs", (route) => {
    requests.push({ key: route.request().headers()["idempotency-key"], body: route.request().postData() });
    return requests.length === 1
      ? json(route, { error: { code: "reconciliation_outcome_unknown" } }, 504)
      : json(route, run, 200, { "Idempotent-Replay": "true" });
  });

  await openReview(page);
  await page.getByRole("button", { name: "Start reconciliation" }).click();
  await expect(page.getByRole("heading", { name: "Reconciliation outcome not confirmed" })).toBeFocused();
  await page.reload();
  await expect(page.getByRole("heading", { name: "Reconciliation outcome not confirmed" })).toBeVisible();
  await page.getByRole("button", { name: "Retry same reconciliation request" }).click();
  await expect(page.getByRole("heading", { name: "Reconciliation passed" })).toBeFocused();
  expect(requests).toHaveLength(2);
  expect(requests[0]).toEqual(requests[1]);
  expect(requests[0].body).toBe("{}");
});

test("request in progress is a stable already-running state with active run evidence", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/reconciliation/runs", (route) => json(route, { error: { code: "request_in_progress" }, run_id: runningRun.run_id }, 409));
  await openReview(page);
  await page.getByRole("button", { name: "Start reconciliation" }).click();
  await expect(page.getByRole("heading", { name: "Reconciliation already running" })).toBeFocused();
  await expect(page.getByText(/parallel run prevented/i)).toBeVisible();
  await expect(page.getByRole("link", { name: "Open running reconciliation" })).toHaveAttribute("href", `/reconciliation/${runningRun.run_id}`);
});

test("polling stops at its fixed deadline and manual refresh remains single-flight", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.addInitScript(({ intent }) => {
    sessionStorage.setItem("ledgersync:reconciliation-command:v1:tenant-1", JSON.stringify(intent));
  }, { intent: {
    version: 1,
    tenantId: "tenant-1",
    idempotencyKey: "reconcile-command-0001",
    state: "running",
    runId: runningRun.run_id,
    submittedAt: "2020-01-01T00:00:00Z",
  } });
  let activeRequests = 0;
  let maximumConcurrentRequests = 0;
  let detailRequests = 0;
  await page.route(`**/api/reconciliation/runs/${runningRun.run_id}`, async (route) => {
    detailRequests += 1;
    activeRequests += 1;
    maximumConcurrentRequests = Math.max(maximumConcurrentRequests, activeRequests);
    await new Promise((resolve) => setTimeout(resolve, 200));
    activeRequests -= 1;
    return json(route, runningRun);
  });

  await page.goto("/reconciliation");
  await expect(page.getByRole("heading", { name: "Reconciliation running" })).toBeVisible();
  await expect(page.getByText(/automatic checking reached its fixed deadline/i)).toBeVisible();
  await page.waitForTimeout(1_100);
  expect(detailRequests).toBe(0);
  const refresh = page.getByRole("button", { name: "Refresh run status" });
  await refresh.click();
  await expect(page.getByRole("button", { name: "Checking run status…" })).toBeDisabled();
  await expect(refresh).toBeEnabled();
  expect(detailRequests).toBe(1);
  expect(maximumConcurrentRequests).toBe(1);
});

test("read-only and offline operators retain history without an enabled run action", async ({ page, context }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/session", (route) => json(route, { subject_id: "reader", tenant_id: "tenant-1", csrf_token: "csrf-test-token", scopes: ["reconciliation:read"], environment: "local" }));
  await page.goto("/reconciliation");
  const action = page.getByRole("button", { name: "Run reconciliation", exact: true });
  await expect(action).toBeDisabled();
  await expect(page.getByText(/read-only role/i)).toBeVisible();
  await expect(page.getByText(run.run_id, { exact: true }).first()).toBeVisible();

  await context.setOffline(true);
  await page.evaluate(() => window.dispatchEvent(new Event("offline")));
  await expect(page.getByText("Reconciliation controls are offline")).toBeVisible();
  await expect(action).toBeDisabled();
});

test("mismatch evidence links affected authorized accounts and never presents all clear", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route(`**/api/reconciliation/runs/${mismatchRun.run_id}`, (route) => json(route, mismatchRun));
  await page.goto(`/reconciliation/${mismatchRun.run_id}`);
  await expect(page.getByRole("heading", { name: "Mismatch detected", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Affected records" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Open affected account" })).toHaveAttribute("href", `/accounts/${sourceAccount.account_id}`);
  await expect(page.getByRole("heading", { name: "Passed", exact: true })).toHaveCount(0);
});

test("review and running controls pass axe and reflow across compact, zoom-equivalent, and forced-color states", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/reconciliation/runs", (route) => json(route, runningRun, 202));
  await page.route(`**/api/reconciliation/runs/${runningRun.run_id}`, (route) => json(route, runningRun));
  await page.setViewportSize({ width: 320, height: 800 });
  await openReview(page);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
  await page.setViewportSize({ width: 640, height: 720 });
  await expect(page.getByRole("region", { name: "Review authoritative reconciliation scope" }).getByText("All authorized INR accounts")).toBeVisible();
  await page.emulateMedia({ forcedColors: "active", reducedMotion: "reduce" });
  await page.getByRole("button", { name: "Start reconciliation" }).click();
  await expect(page.getByRole("heading", { name: "Reconciliation running" })).toBeFocused();
  await page.emulateMedia({ forcedColors: "none", reducedMotion: "reduce" });
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
});
