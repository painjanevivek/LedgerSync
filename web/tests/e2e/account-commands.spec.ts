import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

import { destinationAccount, mockOperatorConsole, sourceAccount } from "./fixtures";

const enteredExternalReference = "SETTLEMENT-INR";
const createdAccount = {
  account_id: "99999999-9999-4999-8999-999999999999",
  tenant_id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
  display_name: "Settlement evidence",
  external_reference: "settlement-inr",
  category: "operating",
  currency: "INR",
  status: "active",
  account_version: "1",
  available_minor: "0",
  ledger_minor: "0",
  created_at: "2026-08-25T10:00:00Z",
  updated_at: "2026-08-25T10:00:00Z",
};

async function advanceToReview(page: import("@playwright/test").Page) {
  await page.getByLabel("Display name").fill(createdAccount.display_name);
  await page.getByLabel("External reference").fill(enteredExternalReference);
  await page.getByLabel("Category").selectOption(createdAccount.category);
  await page.getByRole("button", { name: "Continue to financial boundary" }).click();
  await expect(page.getByText("INR 0.00 · exact")).toBeVisible();
  await page.getByRole("button", { name: "Continue to review" }).click();
}

test("create account preserves directory context and proves the zero-INR boundary", async ({ page }) => {
  await mockOperatorConsole(page);
  let requestBody: unknown;
  let idempotencyKey = "";
  await page.route("**/api/me/accounts", async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    requestBody = route.request().postDataJSON();
    idempotencyKey = route.request().headers()["idempotency-key"] ?? "";
    return route.fulfill({ status: 201, contentType: "application/json", headers: { "X-Request-ID": "request-ref-1" }, body: JSON.stringify(createdAccount) });
  });
  await page.goto("/accounts?q=settlement&status=active");
  await page.getByRole("link", { name: "Create account" }).click();
  await expect(page).toHaveURL(/\/accounts\/new\?return_to=/);
  await advanceToReview(page);
  await expect(page.getByText("This command moves").locator("..")) .toContainText("No money");
  await page.getByRole("button", { name: "Create account", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Account created" })).toBeFocused();
  expect(requestBody).toEqual({ display_name: createdAccount.display_name, external_reference: createdAccount.external_reference, category: createdAccount.category, currency: "INR" });
  expect(idempotencyKey.length).toBeGreaterThanOrEqual(16);
  await expect(page.getByText("Request reference").locator("..")) .toContainText("request-ref-1");
  await expect(page.getByRole("link", { name: "Fund account" })).toHaveAttribute("href", new RegExp(`destination=${createdAccount.account_id}`));
});

test("unknown create result survives reload and retries the exact body and key", async ({ page }) => {
  await mockOperatorConsole(page);
  const submissions: Array<{ body: string | null; key?: string }> = [];
  await page.route("**/api/me/accounts", async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    submissions.push({ body: route.request().postData(), key: route.request().headers()["idempotency-key"] });
    return route.fulfill({ status: 504, contentType: "application/json", body: JSON.stringify({ error: { code: "account_command_outcome_unknown" } }) });
  });
  await page.goto("/accounts/new");
  await advanceToReview(page);
  await page.getByRole("button", { name: "Create account", exact: true }).click();
  await expect(page.getByText("Editing is locked while this exact outcome is unresolved.")).toBeVisible();
  await page.reload();
  await expect(page.getByRole("button", { name: "Retry same account command" })).toBeVisible();
  await page.getByRole("button", { name: "Retry same account command" }).click();
  expect(submissions).toHaveLength(2);
  expect(submissions[1]).toEqual(submissions[0]);
});

test("duplicate create rejection unlocks editing and a replay is labeled explicitly", async ({ page }) => {
  await mockOperatorConsole(page);
  let replay = false;
  const keys: string[] = [];
  await page.route("**/api/me/accounts", (route) => {
    if (route.request().method() !== "POST") return route.continue();
    keys.push(route.request().headers()["idempotency-key"] ?? "");
    return replay
      ? route.fulfill({ status: 201, contentType: "application/json", headers: { "Idempotent-Replay": "true" }, body: JSON.stringify(createdAccount) })
      : route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ error: { code: "external_reference_conflict" } }) });
  });
  await page.goto("/accounts/new");
  await advanceToReview(page);
  await page.getByRole("button", { name: "Create account", exact: true }).click();
  await expect(page.getByText("That external reference already belongs to an account in this tenant.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Back to edit" })).toBeVisible();
  replay = true;
  await page.getByRole("button", { name: "Create account", exact: true }).click();
  await expect(page.getByText("Existing result safely replayed")).toBeVisible();
  expect(keys).toHaveLength(2);
  expect(keys[1]).not.toBe(keys[0]);
});

test("close account binds authoritative account_version, reason and typed reference", async ({ page }) => {
  await mockOperatorConsole(page);
  let currentAccount = { ...sourceAccount, available_minor: "0", ledger_minor: "0", account_version: "7", version: "19" };
  await page.unroute(/\/api\/accounts\/[^/?]+(?:\?.*)?$/);
  await page.route(/\/api\/accounts\/[^/?]+(?:\?.*)?$/, async (route) => {
    if (route.request().method() === "PATCH") {
      expect(route.request().postDataJSON()).toEqual({ expected_version: "7", target_status: "closed", reason: "Retired after exact zero review" });
      currentAccount = { ...currentAccount, status: "closed", account_version: "8" };
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ ...createdAccount, ...currentAccount, tenant_id: createdAccount.tenant_id }) });
    }
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({
      ...currentAccount,
      audit_context: currentAccount.status === "closed" ? [{ event_id: "audit-close-1", event_type: "account.status_changed", actor_subject_id: "operator-1", outcome: "succeeded", correlation_id: "correlation-close-1", reason: "Retired after exact zero review", occurred_at: "2026-08-25T12:00:00Z" }] : [],
    }) });
  });
  await page.unroute("**/api/accounts/*/balance");
  await page.route("**/api/accounts/*/balance", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ account_id: sourceAccount.account_id, currency: "INR", available_minor: "0", ledger_minor: "0", version: "19", as_of: currentAccount.as_of }) }));
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  await page.getByRole("button", { name: "Close account" }).click();
  await page.getByLabel("Reason").fill("Retired after exact zero review");
  await page.getByLabel("Confirm external reference").fill(sourceAccount.external_reference);
  await page.getByRole("button", { name: "Confirm close account" }).click();
  await expect(page.getByText("Account lifecycle is terminal")).toBeVisible();
  await expect(page.getByText("Audited reason:").locator("..")).toContainText("Retired after exact zero review");
});

test("account create states remain accessible and retain draft through compact and zoom-equivalent reflow", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/me/accounts", (route) => route.request().method() === "POST"
    ? route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify(createdAccount) })
    : route.continue());
  await page.setViewportSize({ width: 320, height: 800 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/accounts/new");
  await page.getByLabel("Display name").fill(createdAccount.display_name);
  await page.getByLabel("External reference").fill(createdAccount.external_reference);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.emulateMedia({ forcedColors: "active", reducedMotion: "reduce" });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.emulateMedia({ forcedColors: "none", reducedMotion: "reduce" });
  await page.setViewportSize({ width: 640, height: 800 });
  await expect(page.getByLabel("Display name")).toHaveValue(createdAccount.display_name);
  await page.getByRole("button", { name: "Continue to financial boundary" }).click();
  await page.getByRole("button", { name: "Continue to review" }).click();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByRole("button", { name: "Create account", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Account created" })).toBeFocused();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test("lifecycle confirmation is keyboard-modal, accessible, and 44px minimum", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  const trigger = page.getByRole("button", { name: "Freeze account" });
  await trigger.focus();
  await trigger.press("Enter");
  await expect(page.getByRole("heading", { name: "Freeze account" })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  const box = await page.getByRole("button", { name: "Confirm freeze account" }).boundingBox();
  expect(box?.height).toBeGreaterThanOrEqual(44);
  await page.getByRole("button", { name: "Confirm freeze account" }).click();
  await expect(page.getByText("Cannot submit lifecycle command").locator("..")) .toBeFocused();
  await expect(page.getByLabel("Reason")).toHaveAttribute("aria-invalid", "true");
  await page.keyboard.press("Escape");
  await expect(trigger).toBeFocused();
  await page.getByRole("button", { name: "Close account" }).click();
  await expect(page.getByText("Available balance").locator("..")) .toContainText("INR 1250.00");
  await expect(page.getByText("Ledger balance").locator("..")) .toContainText("INR 1250.00");
  await expect(page.getByRole("button", { name: "Confirm close account" })).toBeDisabled();
  await page.getByRole("button", { name: "Cancel" }).click();
});

test("fund destination applies after async accounts load but never replaces retained transfer intent", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto(`/transfers?destination=${sourceAccount.account_id}`);
  await expect(page.getByLabel("To account")).toHaveValue(sourceAccount.account_id);
  await page.evaluate(({ tenantId, source, destination }) => sessionStorage.setItem(`ledgersync.transfer.intent.${tenantId}`, JSON.stringify({ version: 1, idempotencyKey: "12345678-1234-4234-8234-123456789012", sourceAccountId: source, destinationAccountId: destination, currency: "INR", amountMinor: "500" })), { tenantId: "tenant-1", source: sourceAccount.account_id, destination: destinationAccount.account_id });
  await page.goto(`/transfers?destination=${sourceAccount.account_id}`);
  await expect(page.getByRole("heading", { name: "Confirm exact transfer" })).toBeVisible();
  await expect(page.getByText(destinationAccount.account_id, { exact: true })).toBeVisible();
});

test("funding fails closed when no different authorized INR source has positive exact minor units", async ({ page }) => {
  await mockOperatorConsole(page);
  const zeroSource = { ...sourceAccount, available_minor: "0", ledger_minor: "0" };
  const zeroDestination = { ...destinationAccount, available_minor: "0", ledger_minor: "0" };
  await page.unroute("**/api/me/accounts?*");
  await page.route("**/api/me/accounts?*", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ accounts: [zeroSource, zeroDestination], next_cursor: "" }) }));
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  await expect(page.getByRole("link", { name: "Fund account" })).toHaveCount(0);
  await expect(page.getByText("No different active, authorized INR source has a positive available balance.")).toBeVisible();
  await page.goto(`/transfers?destination=${sourceAccount.account_id}`);
  await expect(page.getByText("No funded source account")).toBeVisible();
  await expect(page.getByRole("button", { name: "Review transfer" })).toHaveCount(0);
});

test("account and transfer scopes independently govern create, lifecycle, and funding", async ({ page }) => {
  await mockOperatorConsole(page);
  async function useScopes(scopes: string[]) {
    await page.unroute("**/api/session");
    await page.route("**/api/session", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ subject_id: "operator-1", tenant_id: "tenant-1", csrf_token: "csrf-test-token", scopes, environment: "local", tenant_label: "My Ledger Workspace", operator_label: "Scoped operator" }) }));
  }

  await useScopes(["accounts:write"]);
  await page.goto("/accounts");
  await expect(page.getByRole("link", { name: "Create account" })).toBeVisible();
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  await expect(page.getByRole("button", { name: "Freeze account" })).toBeEnabled();
  await expect(page.getByRole("link", { name: "Fund account" })).toHaveCount(0);

  await useScopes(["transfers:write"]);
  await page.goto("/accounts");
  await expect(page.getByRole("link", { name: "Create account" })).toHaveCount(0);
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  await expect(page.getByRole("button", { name: "Freeze account" })).toBeDisabled();
  await expect(page.getByRole("link", { name: "Fund account" })).toBeVisible();

  await useScopes([]);
  await page.goto("/accounts");
  await expect(page.getByRole("link", { name: "Create account" })).toHaveCount(0);
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  await expect(page.getByRole("button", { name: "Freeze account" })).toBeDisabled();
  await expect(page.getByRole("link", { name: "Fund account" })).toHaveCount(0);
});

test("detail funding fails closed when the bounded authorized picker is incomplete", async ({ page }) => {
  await mockOperatorConsole(page);
  const requestedLimits: string[] = [];
  await page.unroute("**/api/me/accounts?*");
  await page.route("**/api/me/accounts?*", (route) => {
    const limit = new URL(route.request().url()).searchParams.get("limit") ?? "";
    requestedLimits.push(limit);
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ accounts: [sourceAccount, destinationAccount], next_cursor: limit === "100" ? "more-authorized-accounts" : "" }) });
  });
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  await expect(page.getByText("The authorized account picker exceeds its bounded scope.")).toBeVisible();
  await expect(page.getByRole("link", { name: "Fund account" })).toHaveCount(0);
  expect(requestedLimits).toContain("100");
  await page.goto("/accounts");
  await expect(page.getByRole("heading", { name: "Accounts", exact: true })).toBeVisible();
  expect(requestedLimits).toContain("25");
});

test("close dialog refreshes account configuration and both balances before enabling confirmation", async ({ page }) => {
  await mockOperatorConsole(page);
  let summaryReads = 0;
  let balanceReads = 0;
  let releaseSummaryRefresh!: () => void;
  const summaryRefreshGate = new Promise<void>((resolve) => { releaseSummaryRefresh = resolve; });
  await page.unroute(/\/api\/accounts\/[^/?]+(?:\?.*)?$/);
  await page.route(/\/api\/accounts\/[^/?]+(?:\?.*)?$/, async (route) => { summaryReads += 1; if (summaryReads > 1) await summaryRefreshGate; return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ ...sourceAccount, account_version: "11", available_minor: "0", ledger_minor: "0" }) }); });
  await page.unroute("**/api/accounts/*/balance");
  await page.route("**/api/accounts/*/balance", (route) => { balanceReads += 1; return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ account_id: sourceAccount.account_id, currency: "INR", available_minor: "0", ledger_minor: "0", version: "23", as_of: sourceAccount.as_of }) }); });
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  await expect(page.getByRole("heading", { name: sourceAccount.display_name })).toBeVisible();
  const before = { summaryReads, balanceReads };
  await page.getByRole("button", { name: "Close account" }).click();
  await expect(page.getByText("Refreshing authoritative evidence")).toBeVisible();
  releaseSummaryRefresh();
  await expect(page.getByText("Refreshing authoritative evidence")).toBeHidden();
  expect(summaryReads).toBeGreaterThan(before.summaryReads);
  expect(balanceReads).toBeGreaterThan(before.balanceReads);
  await expect(page.getByText("Available balance").locator("..")) .toContainText("INR 0.00");
  await expect(page.getByText("Ledger balance").locator("..")) .toContainText("INR 0.00");
  await expect(page.getByText("Expected account version").last().locator("..")) .toContainText("11");
  await expect(page.getByRole("button", { name: "Confirm close account" })).toBeEnabled();
});

test("unknown lifecycle outcome locks fields and retries exact body and key inside the active dialog", async ({ page }) => {
  await mockOperatorConsole(page);
  const submissions: Array<{ body: string | null; key?: string }> = [];
  await page.unroute(/\/api\/accounts\/[^/?]+(?:\?.*)?$/);
  await page.route(/\/api\/accounts\/[^/?]+(?:\?.*)?$/, (route) => {
    if (route.request().method() === "PATCH") {
      submissions.push({ body: route.request().postData(), key: route.request().headers()["idempotency-key"] });
      return route.fulfill({ status: 504, contentType: "application/json", body: JSON.stringify({ error: { code: "account_command_outcome_unknown" } }) });
    }
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(sourceAccount) });
  });
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  await page.getByRole("button", { name: "Freeze account" }).click();
  await page.getByLabel("Reason").fill("Reviewing duplicate instructions");
  await page.getByRole("button", { name: "Confirm freeze account" }).click();
  const outcomeRegion = page.getByRole("region", { name: "Lifecycle command outcome" });
  await expect(outcomeRegion).toBeFocused();
  await expect(page.getByLabel("Reason")).toBeDisabled();
  await page.getByRole("button", { name: "Retry same freeze account" }).click();
  expect(submissions).toHaveLength(2);
  expect(submissions[1]).toEqual(submissions[0]);
});

test("account version conflict closes stale confirmation and refreshes current evidence", async ({ page }) => {
  await mockOperatorConsole(page);
  let version = "1";
  let summaryReads = 0;
  await page.unroute(/\/api\/accounts\/[^/?]+(?:\?.*)?$/);
  await page.route(/\/api\/accounts\/[^/?]+(?:\?.*)?$/, (route) => {
    if (route.request().method() === "PATCH") {
      version = "2";
      return route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ error: { code: "account_version_conflict" } }) });
    }
    summaryReads += 1;
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ ...sourceAccount, account_version: version }) });
  });
  await page.goto(`/accounts/${sourceAccount.account_id}`);
  const readsBefore = summaryReads;
  await page.getByRole("button", { name: "Freeze account" }).click();
  await page.getByLabel("Reason").fill("Review after policy update");
  await page.getByRole("button", { name: "Confirm freeze account" }).click();
  await expect(page.getByRole("heading", { name: "Lifecycle command not completed" })).toBeFocused();
  await expect(page.getByText("The account changed after this page loaded.")).toBeVisible();
  expect(summaryReads).toBeGreaterThan(readsBefore + 1);
  await expect(page.getByText("Lifecycle commands use configuration version").locator("..")) .toContainText("2");
});

test("lifecycle dialog reflows at compact, tablet, and desktop sizes with forced colors", async ({ page }) => {
  await mockOperatorConsole(page);
  for (const viewport of [{ width: 320, height: 800 }, { width: 768, height: 1024 }, { width: 1440, height: 900 }]) {
    await page.setViewportSize(viewport);
    await page.emulateMedia({ forcedColors: viewport.width === 320 ? "active" : "none", reducedMotion: "reduce" });
    await page.goto(`/accounts/${sourceAccount.account_id}`);
    await page.getByRole("button", { name: "Freeze account" }).click();
    await expect(page.getByRole("heading", { name: "Freeze account" })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    const confirm = page.getByRole("button", { name: "Confirm freeze account" });
    expect((await confirm.boundingBox())?.height).toBeGreaterThanOrEqual(44);
    await page.keyboard.press("Escape");
    await expect(page.getByRole("button", { name: "Freeze account" })).toBeFocused();
  }
});
