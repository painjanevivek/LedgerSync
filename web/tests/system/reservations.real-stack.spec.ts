import AxeBuilder from "@axe-core/playwright";
import { writeFileSync } from "node:fs";
import { expect, test } from "@playwright/test";
import { requireIsolatedRealStack } from "./real-stack";
test("@real-stack reservation original intent recovers and captures once with exact balance versions", async ({ page }, testInfo) => {
    const run = requireIsolatedRealStack();
    await page.goto("/sign-in");
    await page.getByRole("link", { name: "Sign in", exact: true }).click();
    await expect(page.getByRole("heading", { name: /^(Overview|Your money at a glance)$/ })).toBeVisible();
    const read = async () => {
        const response = await page.request.get(`/api/accounts/${run.sourceAccountID}/balance`);
        expect(response.status()).toBe(200);
        return await response.json() as {
            ledger_minor: string;
            available_minor: string;
            version: string;
        };
    };
    const before = await read();
    expect(BigInt(before.available_minor)).toBeGreaterThanOrEqual(100n);
    await page.goto("/reservations");
    await page.getByLabel("Reservation source account").fill(run.sourceAccountID);
    await page.getByLabel("Reservation destination account").fill("10000000-0000-4000-8000-000000000002");
    await page.getByLabel("Reservation amount in minor units").fill("100");
    await page.getByRole("button", { name: "Review reservation", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Confirm reservation", exact: true })).toBeFocused();
    const attempts: Array<{
        key: string | undefined;
        body: string | null;
    }> = [];
    let lose = true;
    const committed: {
        id?: string;
    } = {};
    await page.route(/\/api\/reservations$/, async (route) => {
        if (route.request().method() !== "POST")
            return route.continue();
        attempts.push({ key: route.request().headers()["idempotency-key"], body: route.request().postData() });
        if (lose) {
            lose = false;
            const response = await route.fetch();
            expect(response.status()).toBe(200);
            committed.id = (await response.json()).id;
            return route.abort("failed");
        }
        return route.continue();
    });
    await page.getByRole("button", { name: "Confirm reserve funds", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Continue the original reservation request", exact: true })).toBeVisible();
    await page.reload();
    await page.getByRole("button", { name: "Retry original reservation request", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Reservation command confirmed", exact: true })).toBeVisible();
    expect(attempts).toHaveLength(2);
    expect(attempts[0]).toEqual(attempts[1]);
    const reserved = await read();
    expect(reserved.ledger_minor).toBe(before.ledger_minor);
    expect(BigInt(reserved.available_minor)).toBe(BigInt(before.available_minor) - 100n);
    expect(BigInt(reserved.version)).toBe(BigInt(before.version) + 1n);
    const obligationExport = await page.request.get("/api/reservations/export.csv");
    expect(obligationExport.status()).toBe(200);
    expect(obligationExport.headers()["x-audit-reference"]).toMatch(/^[0-9a-f-]{36}$/);
    expect(await obligationExport.text()).toContain('"active","100","100","INR"');
    await page.getByRole("button", { name: "Review capture", exact: true }).click();
    const captureReply = page.waitForResponse(response => response.request().method() === "POST" && response.url().endsWith("/decisions"));
    await page.getByRole("button", { name: "Capture full amount", exact: true }).click();
    const captured = await captureReply;
    expect(captured.status()).toBe(200);
    const receipt = await captured.json();
    expect(receipt.state).toBe("captured");
    expect(receipt.id).toBe(committed.id);
    const after = await read();
    expect(BigInt(after.ledger_minor)).toBe(BigInt(before.ledger_minor) - 100n);
    expect(after.available_minor).toBe(reserved.available_minor);
    expect(BigInt(after.version)).toBe(BigInt(before.version) + 2n);
    const current = await page.request.get(`/api/reservations/${receipt.id}`);
    expect(current.status()).toBe(200);
    expect((await current.json()).state).toBe("captured");
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
    await page.setViewportSize({ width: 320, height: 900 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
    await page.screenshot({ path: testInfo.outputPath("reservation-captured-320.png"), fullPage: true });
    writeFileSync(testInfo.outputPath("reservation-proof.json"), JSON.stringify({ scope: "isolated_local_synthetic", hold_id: receipt.id, transfer_id: receipt.transfer_id, original_key_preserved: true, before, reserved, after }, null, 2));
});
