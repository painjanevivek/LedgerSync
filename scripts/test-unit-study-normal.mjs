import { createRequire } from "node:module";
import { writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const require = createRequire(resolve(root, "web/package.json"));
const { chromium } = require("@playwright/test");
const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({ baseURL: "http://127.0.0.1:3000", viewport: { width: 1440, height: 900 } });
const page = await context.newPage();
const pages = [];
try {
  await page.goto("/api/auth/sign-in?return_to=%2F");
  for (const path of ["/", "/accounts", "/funding", "/transfers", "/approvals", "/corrections", "/reconciliation", "/events", "/webhooks", "/developer", "/recovery", "/local-status", "/search", "/guide"]) {
    const response = await page.goto(path);
    if (response?.status() !== 200) throw new Error(`Normal page unavailable: ${path}`);
    await page.getByRole("button", { name: "Sign out", exact: true }).waitFor();
    await page.locator("main h1").waitFor();
    const heading = await page.locator("main h1").allTextContents();
    if (heading.length !== 1) throw new Error(`Normal page heading count: ${path}`);
    pages.push({ path, status: response.status(), heading: heading[0] });
  }
  const response = await context.request.get("/api/developer/distributed");
  if (response.status() !== 200) throw new Error("Normal observations are unavailable.");
  const observations = await response.json();
  if (observations.api_replicas.state !== "unavailable" || observations.api_replicas.count !== undefined || observations.scheduler.state !== "not_observed") {
    throw new Error("Normal observations inferred a study replica count or an active scheduler.");
  }
  await writeFile(resolve(root, "docs/release-evidence/unit-3-unit-4/normal-browser-final.json"), JSON.stringify({ captured_at_utc: new Date().toISOString(), base_url: "http://127.0.0.1:3000", pages, observations, financial_commands: 0 }, null, 2) + "\n");
  console.log(`All ${pages.length} normal pages rendered. Study replica counts are unavailable. The scheduler is not observed.`);
} finally {
  await context.close();
  await browser.close();
}
