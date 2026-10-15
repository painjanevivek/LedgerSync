"use client";

import { ArrowClockwise } from "@phosphor-icons/react";

import { Button } from "@/ui/controls/Button";

export function FocusedRetry({ label, onRetry, disabled = false, busy = false }: Readonly<{ label: string; onRetry: () => void; disabled?: boolean; busy?: boolean }>) {
  return <Button variant="secondary" guarded type="button" disabled={disabled} busy={busy} busyLabel="Retrying…" onClick={onRetry}><ArrowClockwise aria-hidden="true" />{label}</Button>;
}
