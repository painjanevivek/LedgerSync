import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

const routineViewSources = [
  "../../src/features/accounts/AccountViews.tsx",
  "../../src/features/accounts/BalanceStatus.tsx",
  "../../src/features/overview/OverviewView.tsx",
  "../../src/features/funding/FundingViews.tsx",
  "../../src/features/funding/FundingConsole.tsx",
  "../../src/features/transfers/TransferViews.tsx",
  "../../src/features/reconciliation/ReconciliationViews.tsx",
  "../../src/features/console/ConsoleShell.tsx",
].map((path) => readFileSync(new URL(path, import.meta.url), "utf8")).join("\n");

test("routine financial workflows use plain record language", () => {
  for (const auditHeavyPhrase of [
    "account evidence",
    "ledger evidence",
    "funding evidence",
    "transfer evidence",
    "reconciliation evidence",
    "export evidence",
  ]) {
    assert.doesNotMatch(routineViewSources, new RegExp(auditHeavyPhrase, "i"));
  }
});

test("proof-oriented transfer and recovery views retain evidence terminology", () => {
  const proofSources = [
    "../../src/features/transfers/TransferEvidenceTimeline.tsx",
    "../../src/features/recovery/RecoveryView.tsx",
  ].map((path) => readFileSync(new URL(path, import.meta.url), "utf8")).join("\n");

  assert.match(proofSources, /Stored evidence chain/);
  assert.match(proofSources, /Recovery evidence/);
});
