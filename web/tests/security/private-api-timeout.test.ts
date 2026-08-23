import assert from "node:assert/strict";
import test from "node:test";

import { isPrivateAPITimeout, privateWriteTimeoutMilliseconds } from "../../src/lib/upstream-outcome";

test("private transfer timeout is bounded and classified as unknown outcome", () => {
  assert.equal(privateWriteTimeoutMilliseconds, 8_000);
  assert.equal(isPrivateAPITimeout(new DOMException("timed out", "TimeoutError")), true);
  assert.equal(isPrivateAPITimeout(new Error("connection refused")), false);
});
