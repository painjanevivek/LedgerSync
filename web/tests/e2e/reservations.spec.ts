import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { mockOperatorConsole, sourceAccount, destinationAccount } from "./fixtures";
import { reservationStorageKey } from "../../src/features/reservations/reservationIntent";
const id = "33333333-3333-4333-8333-333333333333", now = "2026-10-16T00:00:00Z";
const initial = { id, debit_account_id: sourceAccount.account_id, credit_account_id: destinationAccount.account_id, amount_minor: "250", currency: "INR", state: "active", version: "1", created_by: "operator-1", created_at: now, expires_at: "2026-10-17T00:00:00Z" };
async function fixture(page: Page, { lose = false, unavailable = false } = {}) {
    await mockOperatorConsole(page);
    let exists = false, first = true;
    const attempts: Array<{
        body: string | null;
        key: string | undefined;
    }> = [];
    await page.route(/\/api\/reservations(?:\/[^?]*)?(?:\?.*)?$/, async (route) => {
        const request = route.request(), url = new URL(request.url());
        const json = (value: unknown, status = 200) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(value) });
        if (request.method() === "GET")
            return unavailable ? json({ error: { code: "reservation_unavailable" } }, 503) : json(url.pathname === "/api/reservations" ? { holds: exists ? [initial] : [] } : initial);
        if (url.pathname === "/api/reservations") {
            attempts.push({ body: request.postData(), key: request.headers()["idempotency-key"] });
            exists = true;
            if (lose && first) {
                first = false;
                return route.abort("failed");
            }
            return json({ id, state: "active", replayed: attempts.length > 1, source_balance_version: "1", expires_at: initial.expires_at });
        }
        return json({ id, state: "active", replayed: false, rejection_code: "capture_policy_denied" });
    });
    return { attempts };
}
async function reviewCreate(page: Page) { await page.getByLabel("Reservation source account").fill(sourceAccount.account_id); await page.getByLabel("Reservation destination account").fill(destinationAccount.account_id); await page.getByLabel("Reservation amount in minor units").fill("250"); await page.getByRole("button", { name: "Review reservation", exact: true }).click(); await expect(page.getByRole("heading", { name: "Confirm reservation", exact: true })).toBeFocused(); }
test("reservation review precedes send and supports keyboard return", async ({ page }) => { const observed = await fixture(page); await page.goto("/reservations"); await reviewCreate(page); expect(observed.attempts).toHaveLength(0); await page.keyboard.press("Escape"); await expect(page.getByRole("button", { name: "Review reservation", exact: true })).toBeFocused(); await reviewCreate(page); await page.getByRole("button", { name: "Confirm reserve funds", exact: true }).click(); await expect(page.getByRole("heading", { name: "Reservation command confirmed", exact: true })).toBeVisible(); expect(observed.attempts).toHaveLength(1); });
test("lost reservation response survives reload with exact original intent", async ({ page }) => { const observed = await fixture(page, { lose: true }); await page.goto("/reservations"); await reviewCreate(page); await page.getByRole("button", { name: "Confirm reserve funds", exact: true }).click(); await expect(page.getByRole("heading", { name: "Continue the original reservation request", exact: true })).toBeVisible(); await page.reload(); await page.getByRole("button", { name: "Retry original reservation request", exact: true }).click(); await expect(page.getByRole("heading", { name: "Reservation command confirmed", exact: true })).toBeVisible(); expect(observed.attempts).toHaveLength(2); expect(observed.attempts[0]).toEqual(observed.attempts[1]); await expect(page.getByRole("heading", { name: "Current reservation evidence", exact: true })).toBeVisible(); });
test("capture denial retains an active reservation; layout and accessibility", async ({ page }, testInfo) => {
    await fixture(page);
    await page.goto("/reservations");
    await reviewCreate(page);
    await page.getByRole("button", { name: "Confirm reserve funds", exact: true }).click();
    await page.getByRole("button", { name: "Review capture", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Confirm full capture", exact: true })).toBeFocused();
    await page.getByRole("button", { name: "Capture full amount", exact: true }).click();
    await expect(page.getByText(/Capture was denied/)).toBeVisible();
    await expect(page.getByRole("region", { name: "Authorized reservation evidence", exact: true })).toContainText("active");
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
    for (const width of [320, 375, 768, 1440]) {
        await page.setViewportSize({ width, height: 900 });
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
        await page.evaluate(()=>window.scrollTo(0,0));
 await page.screenshot({ path: testInfo.outputPath(`reservation-${width}.png`), fullPage: true });
    }
});
test("unavailable reservation evidence does not claim an empty result", async ({ page }) => { await fixture(page, { unavailable: true }); await page.goto("/reservations"); await expect(page.getByRole("heading", { name: "Reservation evidence unavailable", exact: true })).toBeVisible(); await expect(page.getByText("No reservations are present on this authorized page.")).toHaveCount(0); });
test("future reservation recovery is preserved and blocks writes", async ({ page }) => { await fixture(page); const key = reservationStorageKey("tenant-1", "operator-1"); await page.addInitScript(({ key }) => sessionStorage.setItem(key, JSON.stringify({ version: 2 })), { key }); await page.goto("/reservations"); await expect(page.getByText(/Saved reservation recovery is invalid/)).toBeVisible(); await expect(page.getByRole("button", { name: "Review reservation", exact: true })).toBeDisabled(); expect(await page.evaluate(key => sessionStorage.getItem(key), key)).toBe('{"version":2}'); });

test("reservation CSV requires an audit reference before saving evidence", async ({ page }) => {
    await fixture(page);
    let audited = false, downloads = 0;
    page.on("download", () => downloads++);
    await page.route("**/api/reservations/export.csv", route => route.fulfill({status:200,contentType:"text/csv; charset=utf-8",headers:audited ? {"X-Audit-Reference":id} : {},body:'"hold_id","state","reserved_minor"\n"'+id+'","active","250"\n'}));
    await page.goto("/reservations");
    const button=page.getByRole("button",{name:"Export reservation obligations CSV",exact:true});
    await expect(button).toBeEnabled();
    await button.click();
    await expect(page.getByText("The download has no valid audit reference.",{exact:true})).toHaveAttribute("role","alert");
    expect(downloads).toBe(0);
    audited=true;
    const saved=page.waitForEvent("download");
    await button.click();
    expect((await saved).suggestedFilename()).toBe("reservation-obligations.csv");
    await expect(page.getByRole("status").filter({hasText:"CSV prepared"})).toBeVisible();
});
