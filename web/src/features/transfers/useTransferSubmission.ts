"use client";

import { useCallback, useMemo, useRef, useState, useSyncExternalStore } from "react";

import type { TransferDetail } from "@/features/accounts/types";
import {
  createStoredTransferIntent,
  parseStoredTransferIntent,
  type PreparedTransfer,
  storedIntentMatches,
  type StoredTransferIntent,
  transferIntentStorageKey,
} from "@/features/transfers/transferIntent";
import type { TransferBalance, TransferResult } from "@/lib/api/transfers";

export type TransferOutcome = Readonly<{
  kind: "success" | "error" | "unknown";
  message: string;
  transferId?: string;
  amountMinor?: string;
  currency?: string;
  source?: string;
  destination?: string;
  occurredAt?: string;
  journalTransactionId?: string;
  balances?: TransferBalance[];
  requestReference?: string;
}> | null;

type TransferErrorPayload = Readonly<{ error?: { code?: string } }>;

function unknownOutcome(reference: string): TransferOutcome {
  return {
    kind: "unknown",
    requestReference: reference,
    message: `The result is not confirmed. Retry this exact transfer; LedgerSync will reuse its existing idempotency key. Request reference: ${reference}.`,
  };
}

function isDefinitiveRejection(status: number, code?: string): boolean {
  if (["insufficient_funds", "transfer_policy_denied", "account_inactive", "idempotency_conflict", "validation_failed", "csrf_failed", "forbidden", "unauthorized"].includes(code ?? "")) return true;
  return status >= 400 && status < 500 && status !== 408 && status !== 429 && code !== "idempotency_in_progress";
}

export function useTransferSubmission(tenantId: string, csrfToken: string, onPosted: () => Promise<void>) {
  const [pending, setPending] = useState(false);
  const inFlight = useRef(false);
  const [outcome, setOutcome] = useState<TransferOutcome>(null);
  const storageKey = transferIntentStorageKey(tenantId);
  const subscribe = useCallback((notify: () => void) => {
    const onStorage = (event: StorageEvent) => {
      if (event.storageArea === sessionStorage && event.key === storageKey) notify();
    };
    window.addEventListener("storage", onStorage);
    window.addEventListener("ledgersync-transfer-intent", notify);
    return () => {
      window.removeEventListener("storage", onStorage);
      window.removeEventListener("ledgersync-transfer-intent", notify);
    };
  }, [storageKey]);
  const getSnapshot = useCallback(() => sessionStorage.getItem(storageKey), [storageKey]);
  const rawStoredIntent = useSyncExternalStore(subscribe, getSnapshot, () => null);
  const storedIntent = useMemo(() => parseStoredTransferIntent(rawStoredIntent), [rawStoredIntent]);

  function notifyIntentChanged() {
    window.dispatchEvent(new Event("ledgersync-transfer-intent"));
  }

  function saveIntent(intent: StoredTransferIntent) {
    sessionStorage.setItem(storageKey, JSON.stringify(intent));
    notifyIntentChanged();
  }

  function clearIntent() {
    sessionStorage.removeItem(storageKey);
    notifyIntentChanged();
  }

  async function loadTransferDetail(transferId: string): Promise<TransferDetail | null> {
    try {
      const response = await fetch(`/api/transfers/${encodeURIComponent(transferId)}`, { cache: "no-store" });
      if (!response.ok) return null;
      const detail = await response.json() as TransferDetail;
      return detail.transfer_id === transferId ? detail : null;
    } catch {
      return null;
    }
  }

  async function submit(prepared: PreparedTransfer) {
    if (pending || inFlight.current) return false;
    inFlight.current = true;

    const persisted = parseStoredTransferIntent(sessionStorage.getItem(storageKey)) ?? storedIntent;
    if (persisted && !storedIntentMatches(persisted, prepared)) {
      setOutcome({
        kind: "unknown",
        message: "A different transfer intent is still unconfirmed. LedgerSync refused to reuse its key. Reload to restore that exact transfer before retrying.",
      });
      inFlight.current = false;
      return false;
    }

    const intent = persisted ?? createStoredTransferIntent(crypto.randomUUID(), prepared);
    const localReference = crypto.randomUUID();
    if (!persisted) saveIntent(intent);
    setPending(true);
    setOutcome(null);

    let response: Response;
    let payload: (TransferResult & TransferErrorPayload) | TransferErrorPayload;
    try {
      response = await fetch("/api/transfers", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": csrfToken,
          "Idempotency-Key": intent.idempotencyKey,
          "X-Request-ID": localReference,
        },
        body: JSON.stringify({
          sourceAccountId: intent.sourceAccountId,
          destinationAccountId: intent.destinationAccountId,
          amount: { currency: intent.currency, minorUnits: intent.amountMinor },
        }),
      });
      payload = await response.json().catch(() => ({})) as (TransferResult & TransferErrorPayload) | TransferErrorPayload;
    } catch {
      setOutcome(unknownOutcome(localReference));
      setPending(false);
      inFlight.current = false;
      return false;
    }

    if (response.ok && "transfer_id" in payload && payload.transfer_id && payload.status === "posted") {
      clearIntent();
      const balances = Object.values(payload.balances ?? {});
      const baseOutcome: TransferOutcome = {
        kind: "success",
        message: "The ledger posting committed exactly once. Committed balance versions are shown below.",
        transferId: payload.transfer_id,
        amountMinor: payload.amount_minor || intent.amountMinor,
        currency: payload.currency || intent.currency,
        source: intent.sourceAccountId,
        destination: intent.destinationAccountId,
        occurredAt: payload.occurred_at,
        balances,
        requestReference: response.headers.get("X-Request-ID") ?? localReference,
      };
      setOutcome(baseOutcome);
      setPending(false);

      const [detail] = await Promise.all([
        loadTransferDetail(payload.transfer_id),
        onPosted().catch(() => undefined),
      ]);
      if (detail) {
        setOutcome({
          ...baseOutcome,
          occurredAt: detail.completed_at || payload.occurred_at,
          journalTransactionId: detail.journal_transaction_id,
        });
      }
      inFlight.current = false;
      return true;
    }

    const code = "error" in payload ? payload.error?.code : undefined;
    const responseReference = response.headers.get("X-Request-ID") ?? localReference;
    if (isDefinitiveRejection(response.status, code)) {
      clearIntent();
      if (code === "insufficient_funds") {
        setOutcome({ kind: "error", requestReference: responseReference, message: `Transfer rejected — insufficient posted balance. No money moved. Request reference: ${responseReference}.` });
      } else if (code === "idempotency_conflict") {
        setOutcome({ kind: "error", requestReference: responseReference, message: `This retry key belongs to a different transfer request. The conflicting local key was cleared; review before creating a new intent. Request reference: ${responseReference}.` });
      } else {
        setOutcome({ kind: "error", requestReference: responseReference, message: `Transfer not posted. The request reached a final rejection, so no unknown movement remains. Request reference: ${responseReference}.` });
      }
    } else {
      setOutcome(unknownOutcome(responseReference));
    }
    setPending(false);
    inFlight.current = false;
    return false;
  }

  const visibleOutcome = outcome ?? (storedIntent ? {
    kind: "unknown" as const,
    message: "An unconfirmed transfer was restored after navigation or reload. Editing is locked; retry this exact intent with its original key.",
  } : null);

  return { outcome: visibleOutcome, pending, setOutcome, storedIntent, submit };
}

export type { PreparedTransfer } from "@/features/transfers/transferIntent";
