import assert from "node:assert/strict";
import test from "node:test";

import { createGuardrailMetric } from "../../src/lib/guardrail-metrics";

test("guardrail metrics expose only bounded non-sensitive dimensions", () => {
  assert.deepEqual(createGuardrailMetric("session", "resolved"), {
    name: "ledgersync.guardrail.outcome",
    kind: "session",
    outcome: "resolved",
  });
  assert.deepEqual(createGuardrailMetric("rate_limit", "tenant-secret:operator-secret"), {
    name: "ledgersync.guardrail.outcome",
    kind: "rate_limit",
    outcome: "unknown",
  });
  assert.deepEqual(Object.keys(createGuardrailMetric("unavailable_action", "offline")).sort(), ["kind", "name", "outcome"]);
});
