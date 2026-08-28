import { defineConfig, devices } from "@playwright/test";

// Offline qualification config for the exact CI container. The caller first
// builds the current tree on the host, then this config serves that immutable
// output without downloading packages or rebuilding inside the Linux image.
export default defineConfig({
  testDir: "./tests/e2e",
  snapshotPathTemplate: "../docs/design/qa/responsive/baselines/{platform}/{projectName}/{arg}{ext}",
  fullyParallel: true,
  reporter: [["list"]],
  use: {
    baseURL: "http://127.0.0.1:3100",
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: "node scripts/prepare-e2e-server.mjs && node -e \"process.env.HOSTNAME='0.0.0.0'; process.env.PORT='3100'; require('./.next/standalone/server.js')\"",
    env: {
      ...process.env,
      LEDGERSYNC_DEPLOYMENT_ENV: "development",
      LEDGERSYNC_PUBLIC_ORIGIN: "http://127.0.0.1:3100",
    },
    url: "http://127.0.0.1:3100",
    reuseExistingServer: false,
    timeout: 60_000,
  },
});
