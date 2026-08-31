"use client";

import { useEffect, useRef, useState, type ReactNode, type RefObject } from "react";

export function FinancialCommandDialog({
  open,
  eyebrow,
  title,
  description,
  confirmLabel,
  busy,
  confirmDisabled = false,
  returnFocusRef,
  restoreTriggerFocus = true,
  onDismiss,
  onConfirm,
  children,
}: Readonly<{
  open: boolean;
  eyebrow: string;
  title: string;
  description: string;
  confirmLabel: string;
  busy: boolean;
  confirmDisabled?: boolean;
  returnFocusRef: RefObject<HTMLButtonElement | null>;
  restoreTriggerFocus?: boolean;
  onDismiss: () => void;
  onConfirm: () => void;
  children: ReactNode;
}>) {
  const dialog = useRef<HTMLDialogElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const [headingId] = useState(() => `financial-command-${crypto.randomUUID()}`);
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

  return (
    <dialog
      ref={dialog}
      className="confirmation-dialog financial-command-dialog"
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
      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (!busy && !confirmDisabled) onConfirm();
        }}
      >
        <p className="eyebrow">{eyebrow}</p>
        <h2 ref={heading} id={headingId} tabIndex={-1}>{title}</h2>
        <p id={descriptionId}>{description}</p>
        {children}
        <div className="action-row account-command-actions">
          <button className="button secondary" type="button" disabled={busy} onClick={onDismiss}>Cancel</button>
          <button className="button danger guarded-control" type="submit" disabled={busy || confirmDisabled}>
            {busy ? "Posting…" : confirmLabel}
          </button>
        </div>
      </form>
    </dialog>
  );
}
