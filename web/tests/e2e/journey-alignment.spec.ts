import { expect, test, type Route } from "@playwright/test";

import { destinationAccount, mockOperatorConsole, sourceAccount, transfer } from "./fixtures";

test.use({ timezoneId: "Asia/Kolkata" });

test("guide includes funding posting and the original-request follow-up path", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/guide");
  await page.getByText("Full operating path", { exact: true }).click();
  const steps = page.getByRole("list", { name: "LedgerSync usage steps", exact: true });
  await expect(steps.locator(":scope > li")).toHaveCount(8);
  const titles = await steps.getByRole("heading").allTextContents();
  expect(titles.indexOf("Post the approved funding")).toBeGreaterThan(titles.indexOf("Review the funding record"));
  expect(titles.indexOf("Post the approved funding")).toBeLessThan(titles.indexOf("Make a transfer"));
  await expect(steps.getByText(/Approval alone does not change the balance/)).toBeVisible();
  await expect(steps.getByText(/keep the original request/)).toBeVisible();
  await page.getByText("Follow up on a record", { exact: true }).click();
  await expect(page.getByRole("link", { name: "Open investigations →", exact: true })).toHaveAttribute("href", "/search");
  await expect(page.getByText(/A saved reference grants no account access/)).toBeVisible();
});

test("account balance timestamps remain UTC in a different browser timezone", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  const timestamp = page.locator(".balance-status time");
  await expect(timestamp).toHaveAttribute("datetime", new Date(sourceAccount.as_of).toISOString());
  await expect(timestamp).toContainText("UTC");
  await expect(timestamp).toContainText("12:00:00");
});

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
}

test("selected transfer evidence precedes history on narrow screens and stays beside it on desktop", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto(`/transfers/${transfer.transfer_id}`);
  const selected = page.getByRole("complementary", { name: `Read-only record for transfer ${transfer.transfer_id}`, exact: true });
  await expect(selected.getByRole("heading", { name: `Transfer ${transfer.transfer_id}`, exact: true })).toBeVisible();
  expect(await selected.evaluate((record) => {
    const history = document.querySelector(".transfer-register-column");
    return history !== null && Boolean(record.compareDocumentPosition(history) & Node.DOCUMENT_POSITION_FOLLOWING);
  })).toBe(true);
  for (const width of [390, 768, 1366]) {
    await page.setViewportSize({ width, height: 900 });
    const record = await selected.boundingBox();
    const history = await page.locator(".transfer-register-column").boundingBox();
    expect(record).not.toBeNull();
    expect(history).not.toBeNull();
    if (width <= 900) expect(record!.y + record!.height).toBeLessThanOrEqual(history!.y);
    else expect(history!.x + history!.width).toBeLessThanOrEqual(record!.x);
  }
});

test("the requested destination opens the transfer form before any financial command", async ({ page }) => {
  await mockOperatorConsole(page);
  let commands = 0;
  await page.route("**/api/transfers", (route) => {
    commands += 1;
    return json(route, {}, 500);
  });
  await page.goto(`/transfers?destination=${destinationAccount.account_id}`);
  await expect(page.getByLabel("To account")).toHaveValue(destinationAccount.account_id);
  await expect(page.getByRole("button", { name: "Review transfer" })).toBeVisible();
  await expect(page.getByRole("button", { name: "New transfer", exact: true })).toHaveCount(0);
  await page.getByLabel("Amount").fill("12.50");
  await page.getByRole("button", { name: "Review transfer" }).click();
  await expect(page.getByRole("heading", { name: "Review transfer", exact: true })).toBeVisible();
  expect(commands).toBe(0);
});

test("secondary navigation remains operable on the transfer screen by keyboard", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockOperatorConsole(page);
  await page.goto("/transfers");
  await page.getByRole("button", { name: "Menu", exact: true }).click();
  const navigation = page.getByRole("navigation", { name: "Primary navigation" });
  const group = navigation.locator("summary").filter({ hasText: "Expert tools" });
  await expect(group).toBeVisible();
  await group.focus();
  await group.press("Enter");
  await expect(navigation.getByRole("link", { name: "Balance checks" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("button", { name: "Menu", exact: true })).toBeFocused();
});

test("account filters precede records and versions remain separate from timestamps", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/accounts");
  const filter = page.getByRole("button", { name: "Apply filters" });
  const comparison = page.getByRole("region", { name: "Authorized account comparison" });
  await expect(filter).toBeVisible();
  await expect(comparison).toBeVisible();
  expect(await filter.evaluate((button) => {
    const records = document.querySelector('[role="region"][aria-label="Authorized account comparison"]');
    return records !== null && Boolean(button.compareDocumentPosition(records) & Node.DOCUMENT_POSITION_FOLLOWING);
  })).toBe(true);
  expect(await page.locator(".account-version-time").first().evaluate((cell) => {
    const version = cell.querySelector("code")?.getBoundingClientRect();
    const timestamp = cell.querySelector("small")?.getBoundingClientRect();
    return Boolean(version && timestamp && timestamp.top - version.bottom >= 4);
  })).toBe(true);
});

test("funding filters appear before the records they control", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/funding");
  const filter = page.locator("summary").filter({ hasText: "Filter records" });
  await expect(filter).toBeVisible();
  await expect(page.getByRole("region", { name: "Funding record comparison" })).toBeVisible();
  expect(await filter.evaluate((summary) => {
    const records = document.querySelector('[role="region"][aria-label="Funding record comparison"]');
    return records !== null && Boolean(summary.compareDocumentPosition(records) & Node.DOCUMENT_POSITION_FOLLOWING);
  })).toBe(true);
  await filter.click();
  await expect(page.getByLabel("Exact funding status")).toBeVisible();
});

test("saved work can be reopened before a search query is entered", async ({ page }) => {
  await mockOperatorConsole(page);
  const paths: string[] = [];
  page.on("request", (request) => paths.push(new URL(request.url()).pathname));
  await page.goto("/search");
  await expect(page.getByRole("heading", { name: "Start with a known record" })).toBeVisible();
  expect(paths).not.toContain("/api/investigation/search");
  expect(paths).not.toContain("/api/investigation/saved-views");
  await page.getByText("Saved views and investigations", { exact: true }).click();
  await expect(page.getByText("Dead delivery events", { exact: true })).toBeVisible();
  await expect(page.getByText("Transfer delivery review", { exact: true })).toBeVisible();
  expect(paths).not.toContain("/api/investigation/search");
});

test("a delivery retry does not imply that a rejected transfer posted money", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route(`**/api/transfers/${transfer.transfer_id}`, (route) => json(route, {
    ...transfer,
    financial_status: "rejected",
    journal_transaction_id: undefined,
    actor_subject_id: "operator-1",
    postings: [],
    timeline: [],
  }));
  await page.goto(`/transfers/${transfer.transfer_id}`);
  await expect(page.getByText("Posting is rejected; delivery is retrying", { exact: true })).toBeVisible();
  await expect(page.getByText(/Money is posted; delivery is/)).toHaveCount(0);
});

test("transfer account links retain the current filter as return context", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/transfers?status=posted");
  const source = page.getByRole("link", { name: `Source account ${sourceAccount.display_name} · ${sourceAccount.account_id}` }).first();
  await expect(source).toHaveAttribute("href", `/accounts/${sourceAccount.account_id}?return_to=${encodeURIComponent("/transfers?status=posted")}`);
  await source.click();
  await page.getByRole("link", { name: "Back to previous view" }).click();
  await expect(page).toHaveURL(/\/transfers\?status=posted$/);
});

test("a missing requested destination blocks preparation instead of choosing a different account", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/transfers?destination=99999999-9999-4999-8999-999999999999");
  await expect(page.getByRole("heading", { name: "Selected destination unavailable" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Review transfer" })).toHaveCount(0);
});

test("a failed transfer refresh retains the result with a historical warning", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto(`/transfers/${transfer.transfer_id}`);
  await expect(page.getByRole("heading", { name: `Transfer ${transfer.transfer_id}`, exact: true })).toBeVisible();
  await page.route(`**/api/transfers/${transfer.transfer_id}`, (route) => json(route, { error: { code: "temporary_unavailable" } }, 503));
  await page.getByRole("button", { name: "Refresh record", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Transfer details not refreshed" })).toBeVisible();
  await expect(page.locator(".transfer-evidence-folio .evidence-freshness.historical")).toBeVisible();
  await expect(page.getByText("Money is posted; delivery is retrying", { exact: true })).toBeVisible();
});

test("initial offline transfer access does not claim an empty verified history", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.addInitScript(() => Object.defineProperty(navigator, "onLine", { get: () => false, configurable: true }));
  await page.goto(`/transfers/${transfer.transfer_id}`);
  await expect(page.getByRole("heading", { name: "Transfer history unavailable", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Transfer details unavailable", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "No transfers found", exact: true })).toHaveCount(0);
});
