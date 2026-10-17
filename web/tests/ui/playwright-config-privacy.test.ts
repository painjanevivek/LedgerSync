import assert from "node:assert/strict";
import test from "node:test";

test("serialised browser configurations omit unrelated inherited credentials", async () => {
  const key = "LEDGERSYNC_UNRELATED_REPORT_SENTINEL";
  const prior = process.env[key];
  const marker = "synthetic-report-privacy-sentinel";
  process.env[key] = marker;
  try {
    const configurations = await Promise.all([
      import("../../playwright.config"),
      import("../../playwright.audit.config"),
      import("../../playwright.pinned-linux.config"),
    ]);
    for (const { default: configuration } of configurations) {
      assert.ok(configuration.webServer && !Array.isArray(configuration.webServer));
      assert.equal(JSON.stringify(configuration).includes(marker), false);
      assert.equal(configuration.webServer.env?.[key], undefined);
    }
  } finally {
    if (prior === undefined) delete process.env[key];
    else process.env[key] = prior;
  }
});
