"use client";

import { useState } from "react";

import type { ReconciliationRun } from "@/features/accounts/types";

export type ReconciliationCommandOutcome =
  | Readonly<{ kind: "run"; run: ReconciliationRun; replayed: boolean; requestReference?: string }>
  | Readonly<{ kind: "already_running"; runId?: string; message: string }>
  | Readonly<{ kind: "unknown" | "unavailable" | "denied" | "error"; code: string; message: string }>;

export function isReconciliationRun(value: unknown): value is ReconciliationRun {
  if (typeof value !== "object" || value === null) return false;
  const run = value as Partial<ReconciliationRun>;
  return typeof run.run_id === "string"
    && typeof run.status === "string"
    && ["matched", "mismatch", "failed", "running"].includes(run.status)
    && typeof run.correlation_id === "string"
    && typeof run.checked_account_count === "string"
    && typeof run.posting_count === "string"
    && typeof run.mismatch_count === "string";
}

function readError(value: unknown): { code: string; runId?: string } {
  if (typeof value !== "object" || value === null) return { code: "temporary_unavailable" };
  const item = value as { error?: { code?: unknown }; run_id?: unknown };
  return {
    code: typeof item.error?.code === "string" ? item.error.code : "temporary_unavailable",
    runId: typeof item.run_id === "string" ? item.run_id : undefined,
  };
}

export function useReconciliationCommand(csrfToken: string) {
  const [pending, setPending] = useState(false);

  async function send(idempotencyKey: string): Promise<ReconciliationCommandOutcome> {
    setPending(true);
    try {
      const response = await fetch("/api/reconciliation/runs", {
        method: "POST",
        cache: "no-store",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": csrfToken,
          "Idempotency-Key": idempotencyKey,
        },
        body: "{}",
      });
      const value: unknown = await response.json().catch(() => ({}));
      if (response.ok && isReconciliationRun(value)) {
        return {
          kind: "run",
          run: value,
          replayed: response.headers.get("Idempotent-Replay") === "true",
          requestReference: response.headers.get("X-Request-ID") ?? undefined,
        };
      }
      const { code, runId } = readError(value);
      if (response.status === 409 && (code === "reconciliation_already_running" || code === "request_in_progress")) {
        return { kind: "already_running", runId, message: "Another authoritative reconciliation is already running for this tenant. Follow that run instead of starting a parallel control." };
      }
      if (response.status === 401 || response.status === 403) {
        return { kind: "denied", code, message: response.status === 401 ? "Your session expired. Sign in again before starting reconciliation." : "Your role is not authorized to run reconciliation." };
      }
      if (response.status === 400 || response.status === 409 || response.status === 415) {
        return { kind: "error", code, message: code === "idempotency_conflict" ? "This retry key is bound to a different request. Inspect retained evidence before starting another run." : "The reconciliation request was rejected before a result could be produced." };
      }
      if (code === "reconciliation_outcome_unknown" || response.status === 504) {
        return { kind: "unknown", code, message: "LedgerSync cannot prove whether the reconciliation started. Retry only this retained request key, or refresh history to locate the authoritative run." };
      }
      return { kind: "unavailable", code, message: response.status === 429 ? "Reconciliation requests are temporarily rate limited. The retained request can be retried after the indicated interval." : "Reconciliation is unavailable. No passing or mismatch result is inferred; retain this request key until authoritative evidence can be checked." };
    } catch {
      return { kind: "unknown", code: "connection_lost", message: "The connection ended after submission. The outcome is unknown; retry only this exact retained request key." };
    } finally {
      setPending(false);
    }
  }

  return { pending, send };
}
