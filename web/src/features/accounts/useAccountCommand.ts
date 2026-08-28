"use client";

import { useRef, useState } from "react";

import type { AccountCommandResult } from "@/features/accounts/types";
import type { CreateAccountFields, LifecycleAccountIntent } from "@/features/accounts/accountCommandIntent";

export type AccountCommandOutcome =
  | Readonly<{ kind: "success"; account: AccountCommandResult; replayed: boolean; requestReference?: string }>
  | Readonly<{ kind: "unknown"; message: string; requestReference?: string }>
  | Readonly<{ kind: "conflict" | "error" | "denied"; code: string; message: string; requestReference?: string }>;

function errorMessage(code: string, status: number): Exclude<AccountCommandOutcome, { kind: "success" }> {
  if (code === "account_version_conflict") return { kind: "conflict", code, message: "The account changed after this page loaded. Current details have been refreshed; review them before trying again." };
  if (code === "external_reference_conflict") return { kind: "conflict", code, message: "That external reference already belongs to an account in this tenant." };
  if (code === "account_not_zero") return { kind: "conflict", code, message: "The account is not exactly zero. Refresh the current balance before closing it." };
  if (code === "invalid_account_transition") return { kind: "conflict", code, message: "The requested lifecycle change is no longer valid for the current account status." };
  if (code === "idempotency_conflict") return { kind: "conflict", code, message: "This retry key is already bound to different account command content. Start a new review." };
  if (status === 401) return { kind: "denied", code, message: "Your session expired. Sign in again before changing an account." };
  if (status === 403) return { kind: "denied", code, message: "Your role is not authorized to change accounts." };
  if (status === 400 || status === 422) return { kind: "error", code, message: "The account command was rejected. Review the exact fields and current account details." };
  return { kind: "unknown", message: "LedgerSync cannot prove whether this account command committed. Retry only the locked command with the same key, or inspect the current account details." };
}

function isCommandResult(value: unknown): value is AccountCommandResult {
  if (typeof value !== "object" || value === null) return false;
  const item = value as Partial<AccountCommandResult>;
  return typeof item.account_id === "string" && typeof item.account_version === "string"
    && typeof item.external_reference === "string" && item.currency === "INR"
    && ["active", "frozen", "closed"].includes(String(item.status));
}

export function useAccountCommand(csrfToken: string) {
  const [pending, setPending] = useState(false);
  const [outcome, setOutcome] = useState<AccountCommandOutcome | null>(null);
  const inFlight = useRef(false);

  async function send(path: string, method: "POST" | "PATCH", request: CreateAccountFields | LifecycleAccountIntent["request"], idempotencyKey: string) {
    if (inFlight.current) return { kind: "unknown" as const, message: "This exact account command is already in flight. Wait for its authoritative response before retrying." };
    inFlight.current = true;
    const localReference = crypto.randomUUID();
    setPending(true);
    setOutcome(null);
    try {
      const response = await fetch(path, {
        method,
        cache: "no-store",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": csrfToken,
          "Idempotency-Key": idempotencyKey,
          "X-Request-ID": localReference,
        },
        body: JSON.stringify(request),
      });
      const value: unknown = await response.json().catch(() => ({}));
      if (response.ok && isCommandResult(value)) {
        const next: AccountCommandOutcome = {
          kind: "success",
          account: value,
          replayed: response.headers.get("Idempotent-Replay") === "true",
          requestReference: response.headers.get("X-Request-ID") ?? localReference,
        };
        setOutcome(next);
        return next;
      }
      const code = typeof value === "object" && value !== null && typeof (value as { error?: { code?: unknown } }).error?.code === "string"
        ? (value as { error: { code: string } }).error.code
        : "temporary_unavailable";
      const reference = response.headers.get("X-Request-ID") ?? localReference;
      const base = code === "account_command_outcome_unknown" || code === "request_in_progress" || response.status >= 500 || response.status === 429
        ? { kind: "unknown" as const, message: "LedgerSync cannot prove whether this account command committed. Retry only the locked command with the same key, or inspect the current account details." }
        : errorMessage(code, response.status);
      const next = { ...base, requestReference: reference, message: `${base.message} Request reference: ${reference}.` };
      setOutcome(next);
      return next;
    } catch {
      const next = { kind: "unknown" as const, requestReference: localReference, message: `The connection ended after submission. The outcome is unknown; retry only this exact locked command with the same key. Request reference: ${localReference}.` };
      setOutcome(next);
      return next;
    } finally { inFlight.current = false; setPending(false); }
  }

  return { pending, outcome, setOutcome, send };
}
