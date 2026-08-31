"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  createFinancialPostIntent,
  financialPostStorageKey,
  parseFinancialPostIntent,
  type FinancialPostDomain,
  type FinancialPostIntent,
} from "@/features/console/financialPostIntent";

export type FinancialPostOutcome<T> =
  | Readonly<{ kind: "success"; record: T; replayed: boolean; requestReference: string }>
  | Readonly<{ kind: "unknown" | "conflict" | "denied" | "error"; message: string; requestReference: string }>;

type Options<T> = Readonly<{
  domain: FinancialPostDomain;
  tenantId: string;
  recordId: string;
  csrfToken: string;
  endpoint: string;
  parseSuccess: (value: unknown) => T | null;
}>;

function errorCode(value: unknown): string {
  return typeof value === "object" && value !== null
    && typeof (value as { error?: { code?: unknown } }).error?.code === "string"
    ? (value as { error: { code: string } }).error.code
    : "temporary_unavailable";
}

export function useFinancialPostCommand<T>({ domain, tenantId, recordId, csrfToken, endpoint, parseSuccess }: Options<T>) {
  const storageKey = useMemo(
    () => financialPostStorageKey(domain, tenantId, recordId),
    [domain, recordId, tenantId],
  );
  const [intent, setIntent] = useState<FinancialPostIntent | null>(null);
  const [outcome, setOutcome] = useState<FinancialPostOutcome<T> | null>(null);
  const [pending, setPending] = useState(false);
  const inFlight = useRef(false);

  const readRecovery = useCallback((): FinancialPostIntent | null => {
    try {
      return parseFinancialPostIntent(
        sessionStorage.getItem(storageKey),
        domain,
        tenantId,
        recordId,
      );
    } catch {
      return null;
    }
  }, [domain, recordId, storageKey, tenantId]);

  function persistRecovery(command: FinancialPostIntent): boolean {
    try {
      sessionStorage.setItem(storageKey, JSON.stringify(command));
      setIntent(command);
      return true;
    } catch {
      return false;
    }
  }

  useEffect(() => {
    if (!tenantId || !recordId) return;
    const timer = window.setTimeout(() => {
      const restored = readRecovery();
      setIntent(restored);
      if (restored) {
        setOutcome({
          kind: "unknown",
          requestReference: restored.idempotencyKey,
          message: "LedgerSync cannot prove whether this permanent posting completed. Refresh the record or retry only this exact command with the retained key.",
        });
      }
    }, 0);
    return () => window.clearTimeout(timer);
  }, [readRecovery, recordId, tenantId]);

  function prepare(): FinancialPostIntent {
    const current = intent?.domain === domain
      && intent.tenantId === tenantId
      && intent.recordId === recordId
      ? intent
      : readRecovery();
    return current ?? createFinancialPostIntent(domain, tenantId, recordId);
  }

  function clearRecovery() {
    try {
      sessionStorage.removeItem(storageKey);
    } catch {
      // State still clears in memory; a stale value is rejected by exact identity parsing.
    }
    setIntent(null);
  }

  async function send(command: FinancialPostIntent): Promise<FinancialPostOutcome<T>> {
    if (inFlight.current) {
      return {
        kind: "unknown",
        requestReference: command.idempotencyKey,
        message: "This exact permanent command is already in flight. Wait for its authoritative response before retrying.",
      };
    }
    const validated = parseFinancialPostIntent(
      JSON.stringify(command),
      domain,
      tenantId,
      recordId,
    );
    const localReference = crypto.randomUUID();
    if (!validated) {
      const next: FinancialPostOutcome<T> = {
        kind: "error",
        requestReference: localReference,
        message: `The permanent command did not match this record and was not sent. Refresh before trying again. Request reference: ${localReference}.`,
      };
      setOutcome(next);
      return next;
    }
    if (!persistRecovery(validated)) {
      const next: FinancialPostOutcome<T> = {
        kind: "error",
        requestReference: localReference,
        message: `Safe retry storage is unavailable, so LedgerSync did not send this permanent command. Restore browser storage and try again. Request reference: ${localReference}.`,
      };
      setOutcome(next);
      return next;
    }
    inFlight.current = true;
    setPending(true);
    setOutcome(null);
    try {
      const response = await fetch(endpoint, {
        method: "POST",
        cache: "no-store",
        headers: {
          "X-CSRF-Token": csrfToken,
          "Idempotency-Key": validated.idempotencyKey,
          "X-Request-ID": localReference,
        },
      });
      const value: unknown = await response.json().catch(() => ({}));
      const requestReference = response.headers.get("X-Request-ID") ?? localReference;
      const record = response.ok ? parseSuccess(value) : null;
      if (record) {
        clearRecovery();
        const next: FinancialPostOutcome<T> = {
          kind: "success",
          record,
          replayed: response.headers.get("Idempotent-Replay") === "true"
            || typeof value === "object" && value !== null && (value as { replayed?: unknown }).replayed === true,
          requestReference,
        };
        setOutcome(next);
        return next;
      }
      const code = errorCode(value);
      const uncertain = response.ok || code.endsWith("_outcome_unknown") || code === "request_in_progress" || response.status === 429 || response.status >= 500;
      if (!uncertain) clearRecovery();
      const kind = uncertain ? "unknown" : response.status === 409 ? "conflict" : response.status === 401 || response.status === 403 || response.status === 428 ? "denied" : "error";
      const message = kind === "unknown"
        ? "LedgerSync cannot prove whether this permanent posting completed. Retry only this exact command with the retained key, or refresh the authoritative record."
        : kind === "conflict"
          ? "The record changed or is no longer approved for posting. Current evidence must be refreshed before another action."
          : kind === "denied"
            ? "This permanent posting is not authorized for the current session. Reauthenticate or ask an authorized operator."
            : "The permanent posting was rejected before LedgerSync could record it. Review the current record before trying again.";
      const next: FinancialPostOutcome<T> = { kind, requestReference, message: `${message} Request reference: ${requestReference}.` };
      setOutcome(next);
      return next;
    } catch {
      const next: FinancialPostOutcome<T> = {
        kind: "unknown",
        requestReference: localReference,
        message: `The connection ended after submission. The outcome is unknown; retry only this exact command with the retained key. Request reference: ${localReference}.`,
      };
      setOutcome(next);
      return next;
    } finally {
      inFlight.current = false;
      setPending(false);
    }
  }

  return { intent, outcome, pending, prepare, send, discard: clearRecovery, setOutcome };
}
