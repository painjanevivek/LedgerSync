"use client";

import { useState } from "react";

import type { AccountCommandResult } from "@/features/accounts/types";
import type { CreateAccountFields, LifecycleAccountIntent } from "@/features/accounts/accountCommandIntent";

export type AccountCommandOutcome =
  | Readonly<{ kind: "success"; account: AccountCommandResult; replayed: boolean; requestReference?: string }>
  | Readonly<{ kind: "unknown"; message: string }>
  | Readonly<{ kind: "conflict" | "error" | "denied"; code: string; message: string }>;

function errorMessage(code: string, status: number): AccountCommandOutcome {
  if (code === "account_version_conflict") return { kind: "conflict", code, message: "The account changed after this page loaded. Current evidence has been refreshed; review it before trying again." };
  if (code === "external_reference_conflict") return { kind: "conflict", code, message: "That external reference already belongs to an account in this tenant." };
  if (code === "account_not_zero") return { kind: "conflict", code, message: "The account is not exactly zero. Refresh current balance evidence before closing it." };
  if (code === "invalid_account_transition") return { kind: "conflict", code, message: "The requested lifecycle change is no longer valid for the current account status." };
  if (code === "idempotency_conflict") return { kind: "conflict", code, message: "This retry key is already bound to different account command content. Start a new review." };
  if (status === 401) return { kind: "denied", code, message: "Your session expired. Sign in again before changing an account." };
  if (status === 403) return { kind: "denied", code, message: "Your role is not authorized to change accounts." };
  if (status === 400 || status === 422) return { kind: "error", code, message: "The account command was rejected. Review the exact fields and current account evidence." };
  return { kind: "unknown", message: "LedgerSync cannot prove whether this account command committed. Retry only the locked command with the same key, or inspect current account evidence." };
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

  async function send(path: string, method: "POST" | "PATCH", request: CreateAccountFields | LifecycleAccountIntent["request"], idempotencyKey: string) {
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
        },
        body: JSON.stringify(request),
      });
      const value: unknown = await response.json().catch(() => ({}));
      if (response.ok && isCommandResult(value)) {
        const next: AccountCommandOutcome = {
          kind: "success",
          account: value,
          replayed: response.headers.get("Idempotent-Replay") === "true",
          requestReference: response.headers.get("X-Request-ID") ?? undefined,
        };
        setOutcome(next);
        return next;
      }
      const code = typeof value === "object" && value !== null && typeof (value as { error?: { code?: unknown } }).error?.code === "string"
        ? (value as { error: { code: string } }).error.code
        : "temporary_unavailable";
      const next = code === "account_command_outcome_unknown" || code === "request_in_progress" || response.status >= 500 || response.status === 429
        ? { kind: "unknown" as const, message: "LedgerSync cannot prove whether this account command committed. Retry only the locked command with the same key, or inspect current account evidence." }
        : errorMessage(code, response.status);
      setOutcome(next);
      return next;
    } catch {
      const next = { kind: "unknown" as const, message: "The connection ended after submission. The outcome is unknown; retry only this exact locked command with the same key." };
      setOutcome(next);
      return next;
    } finally { setPending(false); }
  }

  return { pending, outcome, setOutcome, send };
}
