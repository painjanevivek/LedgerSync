import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";

import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

import { requireIsolatedJourney } from "./real-stack";

async function login(page: Page, origin: string) {
  await page.goto(origin);
  await page.getByRole("link", { name: "Log in", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
}

async function handoff(page: Page, subject: string) {
  await page.getByText("Ownership, handoff, and lifecycle", { exact: true }).click();
  await page.getByText("Hand off to another operator", { exact: true }).click();
  await page.getByLabel("Recipient subject ID").fill(subject);
  await page.getByRole("button", { name: "Confirm ownership handoff" }).click();
  await expect(page.getByRole("heading", { name: "Investigation handed off", exact: true })).toBeVisible();
}

test("@real-stack saved filters, audited bundle, and reauthorized peer handoff", async ({ page, browser }, testInfo) => {
  test.setTimeout(120_000);
  page.setDefaultTimeout(10_000);
  const run = requireIsolatedJourney();
  if (new URL(run.baseURL).port !== "3102") throw new Error("This peer journey requires the isolated 3102/3103 pair");
  const peerContainer = execFileSync("docker", ["ps", "--filter", "publish=3103", "--format", "{{.Label \"com.docker.compose.project\"}}|{{.Label \"com.docker.compose.service\"}}"], { encoding: "utf8", windowsHide: true }).trim();
  expect(peerContainer).toBe(`${run.composeProject}|web-peer`);
  const runID = execFileSync("docker", ["exec", run.postgresContainerID, "psql", "-X", "-U", "ledgersync", "-d", "ledgersync", "-At", "-c", "SELECT id FROM reconciliation_runs WHERE status='matched' ORDER BY completed_at DESC LIMIT 1"], { encoding: "utf8", windowsHide: true }).trim();
  expect(runID).toMatch(/^[0-9a-f-]{36}$/i);
  const peerOrigin = run.baseURL.replace(":3102", ":3103");
  const peerContext = await browser.newContext();
  const peer = await peerContext.newPage();
  peer.setDefaultTimeout(10_000);
  try {
    await login(page, run.baseURL);
    await login(peer, peerOrigin);
    await page.goto(`${run.baseURL}/transfers?status=posted`);
    await page.getByText("Filter history", { exact: false }).first().click();
    await page.getByText("Save current filters", { exact: true }).click();
    await page.getByLabel("Saved view name").fill(`Posted review ${Date.now().toString(36)}`);
    await page.getByRole("button", { name: "Save view", exact: true }).click();
    await expect(page.locator(".saved-view-success")).toBeVisible();
    await page.goto(`${run.baseURL}/search?q=${runID}`);
    const result = page.locator(".investigation-result").first();
    await result.getByText("Preserve as investigation", { exact: true }).click();
    await result.getByLabel("Safe investigation title").fill("Reconciliation follow up");
    await result.getByRole("button", { name: "Create investigation workspace", exact: true }).click();
    await expect(page).toHaveURL(/\/investigations\/[0-9a-f-]{36}$/);
    const workspaceURL = page.url();
    const id = workspaceURL.split("/").at(-1)!;
    await expect(page.getByRole("heading", { name: "Reconciliation follow up", exact: true })).toBeVisible();
    await expect(page.getByRole("region", { name: "Current evidence", exact: true })).toContainText("matched");
    await page.screenshot({ path: testInfo.outputPath("investigation.png"), fullPage: true });
    await page.getByText("Export evidence bundle", { exact: true }).click();
    await page.getByRole("button", { name: "Review evidence bundle", exact: true }).click();
    const response = page.waitForResponse((value) => value.url().endsWith("/evidence-bundle") && value.request().method() === "POST");
    const download = page.waitForEvent("download");
    await page.getByRole("dialog", { name: "Review investigation bundle" }).getByRole("button", { name: "Generate audited ZIP" }).click();
    const bundle = await response;
    expect(bundle.status()).toBe(200);
    expect(bundle.headers()["x-ledgersync-bundle-schema"]).toBe("1");
    expect(bundle.headers()["cache-control"]).toContain("no-store");
    const downloaded = await download;
    expect(downloaded.suggestedFilename()).toMatch(/^ledgersync-investigation-.*-v1\.zip$/);
    const bundlePath = testInfo.outputPath("investigation-bundle.zip");
    await downloaded.saveAs(bundlePath);
    const archive = readFileSync(bundlePath);
    expect(archive.subarray(0, 2).toString()).toBe("PK");
    expect(archive.byteLength).toBe(Number(bundle.headers()["content-length"]));
    await expect(page.getByRole("heading", { name: "Bundle download started", exact: true })).toBeVisible();
    await handoff(page, "local-peer");
    expect((await page.request.get(`${run.baseURL}/api/investigation/workspaces/${id}`)).status()).toBe(404);
    await peer.goto(workspaceURL.replace(":3102", ":3103"));
    await expect(peer.getByRole("heading", { name: "Reconciliation follow up", exact: true })).toBeVisible();
    await expect(peer.getByRole("region", { name: "Current evidence", exact: true })).toContainText("matched");
    await peer.screenshot({ path: testInfo.outputPath("peer-handoff.png"), fullPage: true });
    await handoff(peer, "local-user");
    expect((await peer.request.get(`${peerOrigin}/api/investigation/workspaces/${id}`)).status()).toBe(404);
    await page.goto(workspaceURL);
    await expect(page.getByRole("heading", { name: "Reconciliation follow up", exact: true })).toBeVisible();
    await page.getByText("Ownership, handoff, and lifecycle", { exact: true }).click();
    await page.getByRole("button", { name: "Close investigation", exact: true }).click();
    await expect(page.getByRole("button", { name: "Reopen investigation", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Reopen investigation", exact: true }).click();
    await expect(page.getByRole("button", { name: "Close investigation", exact: true })).toBeVisible();
    const accessibility = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"]).analyze();
    expect(accessibility.violations).toEqual([]);
  } finally { await peerContext.close(); }
});
