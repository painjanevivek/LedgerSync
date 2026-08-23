import assert from "node:assert/strict";
import test from "node:test";

import { getPrivateAPIWorkloadCredential } from "../../src/lib/workload-credential";

test("production refuses a static private API token", async () => {
  const previousEnvironment = process.env.LEDGERSYNC_ENV;
  const previousToken = process.env.LEDGERSYNC_PRIVATE_API_TOKEN;
  const previousFile = process.env.LEDGERSYNC_PRIVATE_API_TOKEN_FILE;
  Object.assign(process.env, { LEDGERSYNC_ENV: "production", LEDGERSYNC_PRIVATE_API_TOKEN: "development-static-token", LEDGERSYNC_PRIVATE_API_TOKEN_FILE: "" });
  try {
    await assert.rejects(getPrivateAPIWorkloadCredential(), /managed renewal/);
  } finally {
    if (previousEnvironment === undefined) delete process.env.LEDGERSYNC_ENV; else process.env.LEDGERSYNC_ENV = previousEnvironment;
    if (previousToken === undefined) delete process.env.LEDGERSYNC_PRIVATE_API_TOKEN; else process.env.LEDGERSYNC_PRIVATE_API_TOKEN = previousToken;
    if (previousFile === undefined) delete process.env.LEDGERSYNC_PRIVATE_API_TOKEN_FILE; else process.env.LEDGERSYNC_PRIVATE_API_TOKEN_FILE = previousFile;
  }
});
