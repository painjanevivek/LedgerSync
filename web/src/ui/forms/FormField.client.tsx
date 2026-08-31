"use client";

import { cloneElement, isValidElement, useId, type ReactElement, type ReactNode } from "react";

type FormControl = ReactElement<{ "aria-describedby"?: string; "aria-invalid"?: boolean | "true" | "false" }>;

export function FormField({ label, requirement, hint, error, errorSummaryId, children }: Readonly<{
  label: string;
  requirement: "required" | "optional";
  hint?: ReactNode;
  error?: ReactNode;
  errorSummaryId?: string;
  children: FormControl;
}>) {
  const fieldId = useId();
  const hintId = hint ? `${fieldId}-hint` : undefined;
  const errorId = error ? `${fieldId}-error` : undefined;
  const describedBy = [children.props["aria-describedby"], hintId, errorId, errorSummaryId].filter(Boolean).join(" ") || undefined;
  const control = isValidElement(children)
    ? cloneElement(children, { "aria-describedby": describedBy, "aria-invalid": error ? true : children.props["aria-invalid"] })
    : children;
  return <div className={`form-field${error ? " invalid" : ""}`}>
    <label className="form-field-label"><span>{label}</span><span className={`field-requirement ${requirement}`}>{requirement === "required" ? "Required" : "Optional"}</span>{control}</label>
    {hint && <p id={hintId} className="form-field-hint">{hint}</p>}
    {error && <p id={errorId} className="form-field-error">{error}</p>}
  </div>;
}
