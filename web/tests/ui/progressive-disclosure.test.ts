import assert from "node:assert/strict";
import { test } from "node:test";

import type { ReconciliationRun } from "../../src/features/accounts/types";
import { deriveWorkspaceStage } from "../../src/features/console/workspaceStage";

const reconciliation = (status: ReconciliationRun["status"], mismatchCount = "0") =>
  ({ status, mismatch_count: mismatchCount }) as ReconciliationRun;

test("workspace stage advances conservatively from authoritative evidence", () => {
  assert.equal(
    deriveWorkspaceStage({ accountCount: 0, transfers: [], reconciliation: null }),
    "empty",
  );
  assert.equal(
    deriveWorkspaceStage({ accountCount: 1, transfers: [], reconciliation: null }),
    "account_ready",
  );
  assert.equal(
    deriveWorkspaceStage({
      accountCount: 1,
      transfers: [{ transfer_id: "transfer-1" }] as never,
      reconciliation: null,
    }),
    "operational",
  );
});

test("workspace attention overrides normal maturity without inventing success", () => {
  assert.equal(
    deriveWorkspaceStage({
      accountCount: 1,
      transfers: [],
      reconciliation: reconciliation("mismatch", "1"),
    }),
    "attention_required",
  );
  assert.equal(
    deriveWorkspaceStage({
      accountCount: 0,
      transfers: [],
      reconciliation: null,
      hasCriticalReadError: true,
    }),
    "attention_required",
  );
});
