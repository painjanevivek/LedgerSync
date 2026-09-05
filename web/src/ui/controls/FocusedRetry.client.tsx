"use client";

import { ArrowClockwise } from "@phosphor-icons/react";

import { Button } from "@/ui/controls/Button";
import { ActionAvailability, type ActionAvailabilityStatus } from "@/ui/controls/ActionAvailability";

export function FocusedRetry({ label, onRetry, disabled = false, busy = false, disabledReason }: Readonly<{ label: string; onRetry: () => void; disabled?: boolean; busy?: boolean; disabledReason?: string }>) {
  const availability: ActionAvailabilityStatus = busy
    ? { state: "busy", reason: "Wait for the current request to finish." }
    : disabled
      ? { state: "temporary_unavailable", reason: disabledReason ?? "Reconnect or restore access before trying again." }
      : { state: "available" };
  return <ActionAvailability availability={availability}><Button variant="secondary" guarded type="button" busy={busy} busyLabel="Retrying…" onClick={onRetry}><ArrowClockwise aria-hidden="true" />{label}</Button></ActionAvailability>;
}
