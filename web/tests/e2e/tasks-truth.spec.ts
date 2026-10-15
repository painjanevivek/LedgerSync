import { expect, test } from "@playwright/test";
import { mockOperatorConsole } from "./fixtures";

test("unavailable browser storage is not invented as a retained transfer", async ({ page }) => {
  await mockOperatorConsole(page, { experienceMode: "simple" });
  await page.addInitScript(() => {
    const original = Storage.prototype.getItem;
    Storage.prototype.getItem = function (key: string) {
      if (key.startsWith("ledgersync.transfer.intent.")) throw new DOMException("Storage denied", "SecurityError");
      return original.call(this, key);
    };
  });
  await page.goto("/tasks");
  await expect(page.getByRole("heading", { name: "Saved browser requests could not be checked" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "An earlier transfer is not confirmed" })).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "No urgent items in the checked records" })).toHaveCount(0);
});

test("task checks never report all clear while requests are unresolved or fail", async ({ page }) => {
  await mockOperatorConsole(page, { experienceMode: "simple" });
  let release!: () => void;
  const held = new Promise<void>((resolve) => { release = resolve; });
  await page.route("**/api/approvals?*", async route => {
    await held;
    await route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "unavailable" } }) });
  });
  await page.goto("/tasks");
  try {
    await expect(page.getByRole("button", { name: "Refreshing…" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "You’re all caught up" })).toHaveCount(0);
  } finally { release(); }
  await expect(page.getByText("Some tasks could not be refreshed").first()).toBeVisible();
  await expect(page.getByRole("heading", { name: "You’re all caught up" })).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Nothing needs your attention" })).toHaveCount(0);
});

test("bounded task history discloses continuation instead of claiming completeness", async ({ page }) => {
  await mockOperatorConsole(page, { experienceMode: "simple" });
  await page.route("**/api/approvals?*", route => route.fulfill({ json: { items: [], page_count: 0, next_cursor: "remaining-approvals" } }));
  await page.route("**/api/transfers?*", route => route.fulfill({ json: { transfers: [], next_cursor: "remaining-transfers" } }));
  await page.route("**/api/reconciliation/runs?*", route => route.fulfill({ json: { runs: [], next_cursor: "" } }));
  await page.route("**/api/funding-events?*", route => route.fulfill({ json: { events: [], next_cursor: "" } }));
  await page.route("**/api/transfer-corrections?*", route => route.fulfill({ json: { events: [], next_cursor: "" } }));
  await page.route("**/api/webhook-endpoints?*", route => route.fulfill({ json: { items: [], next_cursor: "" } }));
  await page.route("**/api/events?*", route => route.fulfill({ json: { events: [], next_cursor: "" } }));
  await page.goto("/tasks");
  await expect(page.getByRole("heading", { name: "More tasks may need attention" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "This task list is incomplete" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Nothing needs your attention" })).toHaveCount(0);
});

test("task aggregation deduplicates approval records and exposes unavailable source coverage", async ({ page }) => {
  await mockOperatorConsole(page, { experienceMode: "simple" });
  await page.route("**/api/events?*", route => route.fulfill({ status: 503, json: { error: { code: "temporary_unavailable" } } }));
  await page.goto("/tasks");
  await expect(page.getByRole("heading", { name: "Review money being added", exact: true })).toHaveCount(1);
  await expect(page.getByText("Some tasks could not be refreshed")).toBeVisible();
  await expect(page.locator(".task-source-coverage").getByText("Could not be checked").first()).toBeVisible();
  await expect(page.getByRole("heading", { name: "No urgent items in the checked records" })).toHaveCount(0);
});
