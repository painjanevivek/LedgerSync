import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { afterEach, test } from "node:test";

import { localCapabilityGuidance } from "../../src/features/operations/LocalStatusView";
import { readJSON, uiDataState, unavailableMessage } from "../../src/lib/api/client";
import type { LocalDiagnostics } from "../../src/lib/api/operations";

const originalFetch = globalThis.fetch;

afterEach(() => { globalThis.fetch = originalFetch; });

test("canonical UI data states never collapse unavailable evidence into ready-empty", () => {
  assert.equal(uiDataState({ loading: true, hasData: false, hasError: false }), "loading");
  assert.equal(uiDataState({ loading: false, hasData: false, hasError: false }), "ready-empty");
  assert.equal(uiDataState({ loading: false, hasData: true, hasError: false }), "ready-populated");
  assert.equal(uiDataState({ loading: false, hasData: true, hasError: true }), "stale");
  assert.equal(uiDataState({ loading: false, hasData: false, hasError: true }), "unavailable");
  assert.equal(uiDataState({ loading: false, hasData: false, hasError: false, forbidden: true }), "forbidden");
  assert.equal(uiDataState({ loading: false, hasData: true, hasError: false, online: false }), "offline");
  assert.equal(uiDataState({ loading: false, hasData: true, hasError: false, partial: true }), "partial");
  assert.equal(uiDataState({ loading: false, hasData: true, hasError: true, partial: true }), "partial");
  assert.equal(uiDataState({ loading: false, hasData: false, hasError: true, unknownAfterSubmit: true }), "unknown-after-submit");
});

test("read failures become bounded unavailable results with a safe request reference", async () => {
  globalThis.fetch = async () => { throw new TypeError("network unavailable"); };
  const result = await readJSON<{ accounts: unknown[] }>("/api/me/accounts");
  assert.equal(result.ok, false);
  assert.equal(result.status, 0);
  assert.equal(result.errorCode, "connection_unavailable");
  assert.match(result.requestReference, /^[A-Za-z0-9-]+$/);
  const message = unavailableMessage(result.status, "accounts", result.requestReference);
  assert.match(message, /Previously verified evidence, if shown, remains historical/);
  assert.match(message, /no empty or successful result is being inferred/);
  assert.match(message, new RegExp(result.requestReference));
});

test("local diagnostics map each failed truth domain to an affected capability and scoped recovery", () => {
  const evidence: LocalDiagnostics = {
    overall_state: "unavailable",
    generated_at: "2026-08-28T10:00:00Z",
    application: { version: "test", commit: "abc", environment: "demo" },
    financial_authority: { postgres: { state: "unavailable" }, latest_reconciliation: { state: "unavailable" } },
    delivery_cache: { outbox: { state: "unavailable", worker_progress: "stalled" }, redis: { state: "unavailable", label: "disposable_cache" } },
  };
  const guidance = localCapabilityGuidance(evidence);
  assert.deepEqual(guidance.map((item) => item.id), ["postgres", "reconciliation-unavailable", "outbox", "redis"]);
  assert.match(guidance[0].capability, /balances, account history, transfers, and reconciliation/);
  assert.match(guidance[2].capability, /committed PostgreSQL money is not reversed/);
  assert.match(guidance[3].recovery, /Never rebuild money from Redis/);
});

test("financial command hooks use immediate in-flight locks and request references", () => {
  for (const path of [
    "../../src/features/accounts/useAccountCommand.ts",
    "../../src/features/transfers/useTransferSubmission.ts",
    "../../src/features/reconciliation/useReconciliationCommand.ts",
  ]) {
    const source = readFileSync(new URL(path, import.meta.url), "utf8");
    assert.match(source, /inFlight\.current/);
    assert.match(source, /"X-Request-ID"/);
    assert.match(source, /crypto\.randomUUID\(\)/);
  }
});

test("modal and compact-navigation focus returns are explicit", () => {
  const lifecycle = readFileSync(new URL("../../src/features/accounts/AccountLifecycleActions.tsx", import.meta.url), "utf8");
  const create = readFileSync(new URL("../../src/features/accounts/AccountCreateFlow.tsx", import.meta.url), "utf8");
  const shell = readFileSync(new URL("../../src/features/console/ConsoleShell.tsx", import.meta.url), "utf8");
  assert.match(lifecycle, /dialogTrigger\.current\?\.focus\(\)/);
  assert.match(create, /abandonTrigger\.current\?\.focus\(\)/);
  assert.match(shell, /menuButton\.current\?\.focus\(\)/);
});
