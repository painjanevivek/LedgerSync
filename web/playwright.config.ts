import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/e2e",
  snapshotPathTemplate: "../docs/design/qa/responsive/baselines/{platform}/{projectName}/{arg}{ext}",
  fullyParallel: true,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: "http://127.0.0.1:3100",
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: "npm run build && node scripts/prepare-e2e-server.mjs && node -e \"process.env.HOSTNAME='0.0.0.0'; process.env.PORT='3100'; require('./.next/standalone/server.js')\"",
    env: { ...process.env, LEDGERSYNC_DEPLOYMENT_ENV: "development", LEDGERSYNC_ENABLE_TEST_RENDER_FAILURE: "true", LEDGERSYNC_PUBLIC_ORIGIN: "http://127.0.0.1:3100" },
    url: "http://127.0.0.1:3100",
    reuseExistingServer: false,
    timeout: 60_000,
  },
});
