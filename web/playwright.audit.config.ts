import { defineConfig } from "@playwright/test";
import base from "./playwright.config";

if (!base.webServer || Array.isArray(base.webServer)) throw new Error("One isolated test frontend is required");

// Same assertions, browser and throttling as the native performance gate.
// Only lifecycle, loopback binding and private report locations differ.
export default defineConfig({
  ...base,
  fullyParallel: false,
  workers: 1,
  use: { ...base.use, baseURL: "http://127.0.0.1:3117" },
  outputDir: "../data/portfolio/audit-remediation-20261017/windows-performance-artifacts",
  reporter: [["list"], ["json", { outputFile: "../data/portfolio/audit-remediation-20261017/windows-performance.json" }]],
  webServer: {
    ...base.webServer,
    command: "npm run build && node scripts/prepare-e2e-server.mjs && node -e \"process.env.HOSTNAME='127.0.0.1'; process.env.PORT='3117'; require('./.next/standalone/server.js')\"",
    url: "http://127.0.0.1:3117",
    env: {
      // Playwright inherits the parent environment. Do not copy it into the
      // serialised config: a JSON reporter would save unrelated credentials.
      LEDGERSYNC_DEPLOYMENT_ENV: "development",
      LEDGERSYNC_PUBLIC_ORIGIN: "http://127.0.0.1:3117",
      LEDGERSYNC_PRIVATE_API_URL: "http://127.0.0.1:9",
      LEDGERSYNC_TELEMETRY_ENABLED: "false",
      LEDGERSYNC_ENABLE_TEST_RENDER_FAILURE: "true",
    },
    reuseExistingServer: false,
  },
});
