"use client";

import { useEffect, useId, useRef, type ReactNode, type RefObject } from "react";

import { Button, type ButtonVariant } from "@/ui/controls/Button";

export function ConfirmationDialog({
  open,
  eyebrow,
  title,
  description,
  confirmLabel,
  busyLabel = "Working…",
  cancelLabel = "Cancel",
  confirmVariant = "danger",
  busy,
  confirmDisabled = false,
  returnFocusRef,
  restoreTriggerFocus = true,
  className = "",
  onDismiss,
  onConfirm,
  children,
}: Readonly<{
  open: boolean;
  eyebrow: string;
  title: string;
  description: string;
  confirmLabel: string;
  busyLabel?: string;
  cancelLabel?: string;
  confirmVariant?: ButtonVariant;
  busy: boolean;
  confirmDisabled?: boolean;
  returnFocusRef: RefObject<HTMLButtonElement | null>;
  restoreTriggerFocus?: boolean;
  className?: string;
  onDismiss: () => void;
  onConfirm: () => void;
  children: ReactNode;
}>) {
  const dialog = useRef<HTMLDialogElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const reactId = useId();
  const headingId = `confirmation-${reactId.replaceAll(":", "")}`;
  const descriptionId = `${headingId}-description`;

  useEffect(() => {
    const element = dialog.current;
    if (!element) return;
    if (open && !element.open) {
      element.showModal();
      requestAnimationFrame(() => heading.current?.focus());
    } else if (!open && element.open) {
      element.close();
    }
  }, [open]);

  return <dialog
    ref={dialog}
    className={["confirmation-dialog", className].filter(Boolean).join(" ")}
    aria-labelledby={headingId}
    aria-describedby={descriptionId}
    onCancel={(event) => {
      event.preventDefault();
      if (!busy) onDismiss();
    }}
    onClose={() => {
      if (restoreTriggerFocus) requestAnimationFrame(() => returnFocusRef.current?.focus());
    }}
  >
    <form onSubmit={(event) => {
      event.preventDefault();
      if (!busy && !confirmDisabled) onConfirm();
    }}>
      <p className="eyebrow">{eyebrow}</p>
      <h2 ref={heading} id={headingId} tabIndex={-1}>{title}</h2>
      <p id={descriptionId}>{description}</p>
      {children}
      <div className="action-row account-command-actions">
        <Button variant="secondary" type="button" disabled={busy} onClick={onDismiss}>{cancelLabel}</Button>
        <Button variant={confirmVariant} guarded type="submit" disabled={confirmDisabled} busy={busy} busyLabel={busyLabel}>{confirmLabel}</Button>
      </div>
    </form>
  </dialog>;
}
