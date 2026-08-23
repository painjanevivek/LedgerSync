"use client";

import { useRef, useState } from "react";

import type { Account } from "@/features/accounts/types";

export type TransferOutcome = {
  kind: "success" | "error" | "unknown";
  message: string;
  transferId?: string;
  amountMinor?: string;
  currency?: string;
  source?: string;
  destination?: string;
} | null;

export type PreparedTransfer = Readonly<{ source: Account; destination: Account; amountMinor: string }>;

function storageKey(tenant: string) { return `ledgersync.transfer.idempotency.${tenant}`; }

export function useTransferSubmission(tenantId: string, csrfToken: string, onPosted: () => Promise<void>) {
  const [pending, setPending] = useState(false);
  const [outcome, setOutcome] = useState<TransferOutcome>(null);
  const idempotencyKey = useRef<string | null>(null);

  async function submit(prepared: PreparedTransfer) {
    if (pending) return false;
    setPending(true);
    setOutcome(null);
    try {
      const stored = sessionStorage.getItem(storageKey(tenantId));
      const requestKey = idempotencyKey.current ?? stored ?? crypto.randomUUID();
      idempotencyKey.current = requestKey;
      if (!stored) sessionStorage.setItem(storageKey(tenantId), requestKey);
      const response = await fetch("/api/transfers", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "Idempotency-Key": requestKey },
        body: JSON.stringify({ sourceAccountId: prepared.source.account_id, destinationAccountId: prepared.destination.account_id, amount: { currency: prepared.source.currency, minorUnits: prepared.amountMinor } }),
      });
      const payload = await response.json().catch(() => ({})) as { transfer_id?: string; error?: { code?: string } };
      if (response.ok && payload.transfer_id) {
        sessionStorage.removeItem(storageKey(tenantId));
        idempotencyKey.current = null;
        setOutcome({ kind: "success", message: "The ledger posting committed exactly once. Affected balances were refreshed.", transferId: payload.transfer_id, amountMinor: prepared.amountMinor, currency: prepared.source.currency, source: prepared.source.account_id, destination: prepared.destination.account_id });
        await onPosted();
        return true;
      }
      if (response.status === 409 && payload.error?.code === "insufficient_funds") {
        setOutcome({ kind: "error", message: "Transfer rejected — insufficient posted balance. No money moved." });
      } else if (response.status === 409 && payload.error?.code === "idempotency_conflict") {
        sessionStorage.removeItem(storageKey(tenantId));
        idempotencyKey.current = null;
        setOutcome({ kind: "error", message: "This retry key belongs to a different transfer request. Return to edit to create a genuinely new intent." });
      } else {
        setOutcome({ kind: "unknown", message: "The result is not confirmed. Retry this same transfer; LedgerSync will reuse the existing idempotency key." });
      }
    } catch {
      setOutcome({ kind: "unknown", message: "The result is not confirmed. Retry this same transfer; LedgerSync will reuse the existing idempotency key." });
    } finally {
      setPending(false);
    }
    return false;
  }

  return { outcome, pending, setOutcome, submit };
}
