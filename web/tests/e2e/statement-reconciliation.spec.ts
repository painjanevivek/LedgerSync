import AxeBuilder from "@axe-core/playwright";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { expect, test, type Page, type Route } from "@playwright/test";
import { mockOperatorConsole, sourceAccount } from "./fixtures";
import { statementStorageKey } from "../../src/features/statement-reconciliation/statementIntent";
const mappingID = "11111111-0000-4000-8000-000000000001", importID = "11111111-0000-4000-8000-000000000002", runID = "11111111-0000-4000-8000-000000000003";
const csv = readFileSync("public/samples/statement-v1.csv"), digest = createHash("sha256").update(csv).digest("hex"), now = "2026-10-16T00:00:00Z";
const mapping = { id: mappingID, account_id: sourceAccount.account_id, source_account_ref: "synthetic-clearing", currency: "INR", version: 1, created_by: "operator-1", created_at: now };
const preview = { sha256: digest, source_account_ref: "synthetic-clearing", period_start: "2026-10-01T00:00:00Z", period_end: "2026-11-01T00:00:00Z", opening_minor: "1000", closing_minor: "850", row_count: 2 };
const imported = { id: importID, mapping_id: mappingID, ...preview, created_at: now };
const row = { kind: "statement", item_id: "synthetic-row-001", version: "0", classification: "unmatched_external", status: "open", external_reference: "synthetic-ref", direction: "debit", amount_minor: "250", currency: "INR", assignee: "", reason: "", comment: "", evidence: "", linked_posting_id: "", updated_by: "operator-1", updated_at: now };
const run = { id: runID, import_id: importID, mapping_id: mappingID, account_id: sourceAccount.account_id, source_account_ref: "synthetic-clearing", state: "completed", rule_version: 1, mapping_version: 1, cutoff: now, period_start: preview.period_start, period_end: preview.period_end, source_sha256: digest, statement_row_count: 2, snapshot_item_count: 0, matched_row_count: 0, unmatched_statement_count: 2, unmatched_ledger_count: 0 };
function json(route: Route, body: unknown, status = 200) { return route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) }); }
async function fixture(page: Page, options: {
    emptyMapping?: boolean;
    loseImport?: boolean;
    unavailable?: boolean;
    accepted?: boolean;
} = {}) {
    await mockOperatorConsole(page);
    let bound = !options.emptyMapping, accepted = options.accepted ?? false, completed = options.accepted ?? false, reviewed = false, firstImport = true;
    const imports: Array<{
        key: string | null;
        body: string | null;
    }> = [];
    await page.route(/\/api\/statement-reconciliation(?:\/[^?]*)?(?:\?.*)?$/, async (route) => {
        const req = route.request(), url = new URL(req.url()), path = url.pathname;
        if (path === "/api/statement-reconciliation" && req.method() === "GET")
            return options.unavailable ? json(route, { error: { code: "statement_unavailable" } }, 503) : json(route, { mapping: bound ? mapping : null, imports: accepted ? [{ ...imported, ...(completed ? { run_id: runID, run_state: "completed" } : {}) }] : [], accounts: [{ id: sourceAccount.account_id, display_name: "Clearing study", kind: "customer" }], more_accounts: false });
        if (path.endsWith("/mappings")) {
            bound = true;
            return json(route, { id: mappingID, replayed: false });
        }
        if (path.endsWith("/preview"))
            return json(route, preview);
        if (path.endsWith("/imports") && req.method() === "POST") {
            imports.push({ key: req.headers()["idempotency-key"] ?? null, body: req.postData() });
            accepted = true;
            if (options.loseImport && firstImport) {
                firstImport = false;
                return route.abort("failed");
            }
            return json(route, { id: importID, replayed: imports.length > 1 });
        }
        if (path.endsWith("/runs") && req.method() === "POST") {
            completed = true;
            return json(route, { id: runID, replayed: false });
        }
        if (path.endsWith("/reviews")) {
            const body = req.postDataJSON() as {
                expected_version: string;
                status: string;
                comment: string;
            };
            expect(body.expected_version).toBe("0");
            expect(body.status).toBe("reviewed_exception");
            reviewed = true;
            return json(route, { id: runID, replayed: false });
        }
        if (path.endsWith("/" + runID))
            return json(route, { run, items: [{ ...row, ...(reviewed ? { version: "1", status: "reviewed_exception", assignee: "operator-1", reason: "investigation", comment: "Difference inspected" } : {}) }] });
        return json(route, { error: { code: "not_found" } }, 404);
    });
    return { imports };
}
test("statement setup is reviewed before its fixed mapping is accepted", async ({ page }) => {
    await fixture(page, { emptyMapping: true });
    await page.goto("/reconciliation/statements");
    await page.getByLabel("Mapped INR account").fill(sourceAccount.account_id);
    await page.getByRole("button", { name: "Review fixed mapping", exact: true }).click();
    await expect(page.getByText("Confirm this fixed mapping.", { exact: false })).toBeVisible();
    await page.getByRole("button", { name: "Confirm fixed mapping", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Command confirmed", exact: true })).toBeVisible();
    await expect(page.getByText("Mapping version 1 stays fixed.", { exact: false })).toBeVisible();
});
test("populated statement imports have unique landmarks and retain narrow-screen evidence", async ({ page },testInfo) => {
    await fixture(page,{accepted:true});
    await page.goto(`/reconciliation/statements?run=${runID}`);
    await expect(page.getByRole("region",{name:"Accepted statement source records",exact:true})).toBeVisible();
    expect((await new AxeBuilder({page}).analyze()).violations).toEqual([]);
    await page.setViewportSize({width:320,height:900});
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=document.documentElement.clientWidth)).toBe(true);
    await page.screenshot({path:testInfo.outputPath("statement-populated-320.png"),fullPage:true});
});
test("lost import survives reload and reuses exact bytes and original key", async ({ page }) => {
    const state = await fixture(page, { loseImport: true });
    await page.goto("/reconciliation/statements");
    await page.getByLabel("Statement CSV").setInputFiles({ name: "synthetic.csv", mimeType: "text/csv", buffer: csv });
    await expect(page.getByRole("button", { name: "Accept immutable source", exact: true })).toBeEnabled();
    await page.getByRole("button", { name: "Accept immutable source", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Continue the original request", exact: true })).toBeVisible();
    await page.reload();
    await expect(page.getByRole("heading", { name: "Continue the original request", exact: true })).toBeVisible();
    await page.getByLabel("Original CSV for recovery").setInputFiles({ name: "same.csv", mimeType: "text/csv", buffer: csv });
    await page.getByRole("button", { name: "Retry original request", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Command confirmed", exact: true })).toBeVisible();
    expect(state.imports).toHaveLength(2);
    expect(state.imports[1]).toEqual(state.imports[0]);
    expect(await page.evaluate(key => sessionStorage.getItem(key), statementStorageKey("tenant-1", "operator-1"))).toBeNull();
});
test("review keeps an unmatched difference visible and supports keyboard focus", async ({ page }, testInfo) => {
    await fixture(page);
    await page.goto(`/reconciliation/statements?run=${runID}`);
    await expect(page.getByRole("region", { name: "Statement comparison review queue", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Review item", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Review statement row", exact: true })).toBeFocused();
    await page.getByLabel("Review status").selectOption("reviewed_exception");
    await page.getByRole("button", { name: "Assign to me", exact: true }).click();
    await page.getByLabel("Comment").fill("Difference inspected");
    await page.getByRole("button", { name: "Save review decision", exact: true }).click();
    await expect(page.getByRole("region", { name: "Statement comparison review queue", exact: true })).toContainText("reviewed exception");
    await expect(page.getByText("0 / 0", { exact: true })).toHaveCount(0);
    await expect(page.getByText("2 / 0", { exact: true })).toBeVisible();
    const axe = await new AxeBuilder({ page }).analyze();
    expect(axe.violations).toEqual([]);
    await page.evaluate(()=>window.scrollTo(0,0));
 await page.screenshot({ path: testInfo.outputPath("statement-desktop.png"), fullPage: true });
    for (const width of [320, 390, 768, 1280]) {
        await page.setViewportSize({ width, height: 900 });
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    }
    await page.setViewportSize({ width: 390, height: 844 });
    await page.screenshot({ path: testInfo.outputPath("statement-mobile.png"), fullPage: true });
});
test("unavailable evidence is not an empty import list", async ({ page }) => {
    await fixture(page, { unavailable: true });
    await page.goto("/reconciliation/statements");
    await expect(page.getByRole("heading", { name: "Statement workspace unavailable", exact: true })).toBeVisible();
    await expect(page.getByText("No accepted statements", { exact: true })).toHaveCount(0);
});
test("unknown saved intent remains intact and blocks new acceptance", async ({ page }) => {
    const key = statementStorageKey("tenant-1", "operator-1");
    await page.addInitScript(({ key }) => sessionStorage.setItem(key, '{"version":99}'), { key });
    await fixture(page);
    await page.goto("/reconciliation/statements");
    await expect(page.getByText("Saved statement recovery is invalid or belongs to another identity. It was preserved.", { exact: true })).toBeVisible();
    await expect(page.getByLabel("Statement CSV")).toBeDisabled();
    expect(await page.evaluate(key => sessionStorage.getItem(key), key)).toBe('{"version":99}');
});
