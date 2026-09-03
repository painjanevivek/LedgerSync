import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { test } from "node:test";

test("the route error boundary retries without disclosing raw failures", () => {
  const source = readFileSync(new URL("../../src/app/error.tsx", import.meta.url), "utf8");

  assert.match(source, /^"use client";/u);
  assert.match(source, /reset\(\);\s*window\.location\.reload\(\);/u);
  assert.match(source, /onClick=\{retryCurrentRoute\}/u);
  assert.match(source, /has not inferred a financial result/u);
  assert.doesNotMatch(source, /\.message\b|\.digest\b|console\.|JSON\.stringify/u);
  assert.doesNotMatch(source, /request[-_ ]?id|correlation[-_ ]?id|tenant[-_ ]?id|subject[-_ ]?id/iu);
});

test("the not-found boundary is non-disclosing and offers one safe destination", () => {
  const source = readFileSync(new URL("../../src/app/not-found.tsx", import.meta.url), "utf8");

  assert.match(source, /Page unavailable/u);
  assert.match(source, /No record or access status is disclosed/u);
  assert.match(source, /href="\/"/u);
  assert.doesNotMatch(source, /tenant|authorization policy|permission|administrator|object exists/iu);
});

test("the render-failure probe is unreachable outside the exact local test boundary", () => {
  const source = readFileSync(new URL("../../src/app/test-support/route-error/page.tsx", import.meta.url), "utf8");

  assert.match(source, /LEDGERSYNC_ENABLE_TEST_RENDER_FAILURE === "true"/u);
  assert.match(source, /LEDGERSYNC_DEPLOYMENT_ENV === "development"/u);
  assert.match(source, /requestHeaders\.get\("host"\) === "127\.0\.0\.1:3100"/u);
  assert.match(source, /attemptPattern\.test\(attempt\)/u);
  assert.match(source, /if \(!enabled\) notFound\(\)/u);
  assert.doesNotMatch(source, /fetch\(|\/api\//u);
});

test("unjustified root and loading boundaries remain absent", () => {
  assert.equal(existsSync(new URL("../../src/app/global-error.tsx", import.meta.url)), false);
  assert.equal(existsSync(new URL("../../src/app/loading.tsx", import.meta.url)), false);
});
