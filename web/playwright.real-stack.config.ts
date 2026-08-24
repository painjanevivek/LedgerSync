import { defineConfig, devices } from "@playwright/test";

import { parseSystemWebURL } from "./tests/system/real-stack-boundary";

const baseURL = process.env.LEDGERSYNC_SYSTEM_WEB_URL
  ? parseSystemWebURL(process.env.LEDGERSYNC_SYSTEM_WEB_URL)
  : "http://127.0.0.1:3000";

export default defineConfig({
  testDir: "./tests/system",
  testMatch: "**/*.real-stack.spec.ts",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 120_000,
  outputDir: "test-results/real-stack",
  reporter: [["list"], ["html", { open: "never", outputFolder: "playwright-report-real-stack" }]],
  use: {
    ...devices["Desktop Chrome"],
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
});
