import AxeBuilder from "@axe-core/playwright";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { readFileSync, writeFileSync } from "node:fs";
import { expect, test } from "@playwright/test";
import { requireIsolatedRealStack, type RealStackRun } from "./real-stack";
function financialProof(run: RealStackRun) {
    const info = JSON.parse(execFileSync("docker", ["inspect", run.postgresContainerID], { encoding: "utf8", windowsHide: true, timeout: 15000 }))[0];
    if (!info.Id.startsWith(run.postgresContainerID) || info.Config.Labels["com.docker.compose.project"] !== run.composeProject || info.Config.Labels["com.docker.compose.service"] !== "postgres" || !/^[0-9a-f]{64}$/.test(info.Config.Labels["ledgersync.candidate"] ?? "") || info.Mounts.some((m: {
        Type: string;
    }) => m.Type !== "tmpfs"))
        throw new Error("Statement proof requires the exact isolated temporary PostgreSQL target");
    const sql = `SELECT json_build_object('accounts',(SELECT count(*) FROM accounts),'transfers',(SELECT count(*) FROM transfers),'postings',(SELECT count(*) FROM ledger_postings),'balances',(SELECT md5(string_agg(account_id::text||':'||available_minor::text||':'||ledger_minor::text||':'||balance_version::text,',' ORDER BY account_id)) FROM account_balance_projections),'transfer_hash',(SELECT md5(string_agg(id::text||':'||status||':'||amount_minor::text||':'||currency,',' ORDER BY id)) FROM transfers),'posting_hash',(SELECT md5(string_agg(id::text||':'||journal_transaction_id::text||':'||account_id::text||':'||direction||':'||amount_minor::text||':'||currency,',' ORDER BY id)) FROM ledger_postings))::text;`;
    return JSON.parse(execFileSync("docker", ["exec", "-i", info.Id, "psql", "-X", "-v", "ON_ERROR_STOP=1", "-U", "ledgersync", "-d", "ledgersync", "-Atc", sql], { encoding: "utf8", windowsHide: true, timeout: 15000 }));
}
test("@real-stack statement import recovers its original key and reviews a difference without financial mutation", async ({ page }, testInfo) => {
    const run = requireIsolatedRealStack(), csv = readFileSync("public/samples/statement-v1.csv"), sha = createHash("sha256").update(csv).digest("hex");
    await page.goto("/sign-in");
    await page.getByRole("link", { name: "Sign in", exact: true }).click();
    await expect(page.getByRole("heading", { name: /^(Overview|Your money at a glance)$/ })).toBeVisible();
    const before = financialProof(run);
    await page.goto("/reconciliation/statements");
    await expect(page.getByRole("heading", { name: "External statement comparison", exact: true })).toBeVisible();
    const sessionResponse = await page.request.get("/api/session");
    expect(sessionResponse.status()).toBe(200);
    const session = await sessionResponse.json();
    const csrfDenial = await page.request.post("/api/statement-reconciliation/preview", { headers: { Origin: "https://other.invalid", "X-CSRF-Token": session.csrf_token, "Idempotency-Key": crypto.randomUUID() }, data: { source_base64: csv.toString("base64") } });
    expect(csrfDenial.status()).toBe(403);
    await page.getByLabel("Mapped INR account").fill(run.sourceAccountID);
    await page.getByRole("button", { name: "Review fixed mapping", exact: true }).click();
    const mappingReply = page.waitForResponse(r => r.request().method() === "POST" && new URL(r.url()).pathname === "/api/statement-reconciliation/mappings");
    await page.getByRole("button", { name: "Confirm fixed mapping", exact: true }).click();
    expect((await mappingReply).status()).toBe(200);
    await page.getByLabel("Statement CSV").setInputFiles({ name: "synthetic-statement.csv", mimeType: "text/csv", buffer: csv });
    await expect(page.getByRole("button", { name: "Accept immutable source", exact: true })).toBeEnabled();
    let lostReceipt: {
        id: string;
    } | null = null;
    const attempts: Array<{
        key: string | undefined;
        body: string | null;
    }> = [];
    let lose = true;
    await page.route("**/api/statement-reconciliation/imports", async (route) => {
        const request = route.request();
        attempts.push({ key: request.headers()["idempotency-key"], body: request.postData() });
        if (lose) {
            lose = false;
            const response = await route.fetch();
            expect(response.status()).toBe(200);
            lostReceipt = await response.json();
            return route.abort("failed");
        }
        return route.continue();
    });
    await page.getByRole("button", { name: "Accept immutable source", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Continue the original request", exact: true })).toBeVisible();
    await page.reload();
    await expect(page.getByRole("heading", { name: "Continue the original request", exact: true })).toBeVisible();
    await page.getByLabel("Original CSV for recovery").setInputFiles({ name: "same-original.csv", mimeType: "text/csv", buffer: csv });
    const replayReply = page.waitForResponse(r => r.request().method() === "POST" && new URL(r.url()).pathname === "/api/statement-reconciliation/imports");
    await page.getByRole("button", { name: "Retry original request", exact: true }).click();
    const replay = await replayReply;
    expect(replay.status()).toBe(200);
    expect(await replay.json()).toEqual({ id: lostReceipt!.id, replayed: true });
    expect(attempts).toHaveLength(2);
    expect(attempts[1]).toEqual(attempts[0]);
    await expect(page.getByRole("heading", { name: "Continue the original request", exact: true })).toHaveCount(0);
    const imports = page.getByRole("region", { name: "Accepted statement sources", exact: true });
    await expect(imports.getByRole("row")).toHaveCount(2);
    const runReply = page.waitForResponse(r => r.request().method() === "POST" && new URL(r.url()).pathname === "/api/statement-reconciliation/runs");
    await page.getByRole("button", { name: "Compare fixed snapshot", exact: true }).click();
    const prepared = await runReply;
    expect(prepared.status()).toBe(200);
    const receipt = await prepared.json();
    await expect(page).toHaveURL(new RegExp(`/reconciliation/statements\\?run=${receipt.id}$`));
    const queue = page.getByRole("region", { name: "Statement comparison review queue", exact: true });
    await expect(queue).toContainText("synthetic-row-001");
    const target = queue.getByRole("row").filter({ hasText: "synthetic-row-001" });
    await target.getByRole("button", { name: "Review item", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Review statement row", exact: true })).toBeFocused();
    await page.getByLabel("Review status").selectOption("reviewed_exception");
    await page.getByRole("button", { name: "Assign to me", exact: true }).click();
    await page.getByLabel("Comment").fill("Synthetic source difference reviewed; no journal adjustment.");
    const reviewReply = page.waitForResponse(r => r.request().method() === "POST" && new URL(r.url()).pathname.endsWith("/reviews"));
    await page.getByRole("button", { name: "Save review decision", exact: true }).click();
    expect((await reviewReply).status()).toBe(200);
    await expect(queue.getByRole("row").filter({ hasText: "synthetic-row-001" })).toContainText("reviewed exception");
    const originalReply = await page.request.get(`/api/statement-reconciliation/imports/${lostReceipt!.id}/source.csv`);
    expect(originalReply.status()).toBe(200);
    expect(originalReply.headers()["x-statement-sha256"]).toBe(sha);
    expect(await originalReply.body()).toEqual(csv);
    expect(originalReply.headers()["x-audit-reference"]).toMatch(/^[0-9a-f-]{36}$/);
    const exportReply = await page.request.get(`/api/statement-reconciliation/runs/${receipt.id}/comparison.csv`);
    expect(exportReply.status()).toBe(200);
    expect(await exportReply.text()).toContain("Synthetic source difference reviewed");
    const after = financialProof(run);
    expect(after).toEqual(before);
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
    await page.screenshot({ path: testInfo.outputPath("statement-real-desktop.png"), fullPage: true });
    await page.setViewportSize({ width: 320, height: 900 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.screenshot({ path: testInfo.outputPath("statement-real-mobile.png"), fullPage: true });
    writeFileSync(testInfo.outputPath("statement-financial-preservation.json"), JSON.stringify({ source_sha256: sha, import_id: lostReceipt!.id, run_id: receipt.id, original_key_reused: attempts[0].key === attempts[1].key, body_reused: attempts[0].body === attempts[1].body, financial_before: before, financial_after: after, financial_preserved: true, source_download_exact: true, reviewed_exception_remains_unmatched: true }, null, 2) + "\n");
});
