import type { ButtonHTMLAttributes } from "react";

export type ButtonVariant = "primary" | "secondary" | "danger";

export function Button({ variant = "secondary", guarded = false, busy = false, busyLabel = "Working…", className = "", children, disabled, ...props }: Readonly<ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  guarded?: boolean;
  busy?: boolean;
  busyLabel?: string;
}>) {
  const classes = ["button", variant, guarded ? "guarded-control" : "", className].filter(Boolean).join(" ");
  return <button {...props} className={classes} disabled={disabled || busy} aria-busy={busy || undefined}>{busy ? busyLabel : children}</button>;
}
