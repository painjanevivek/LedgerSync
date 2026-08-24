"use client";

import { ArrowRight, Check, CheckCircle, Copy, Info, WarningCircle, XCircle } from "@phosphor-icons/react";
import Link from "next/link";
import { useState, type ReactNode } from "react";

export function CopyControl({ value, label = "Copy identifier" }: Readonly<{ value: string; label?: string }>) {
  const [state, setState] = useState<"idle" | "copied" | "failed">("idle");
  async function copy() { try { await navigator.clipboard.writeText(value); setState("copied"); } catch { setState("failed"); } window.setTimeout(() => setState("idle"), 1800); }
  return <span className="copy-control"><code title={value}>{value}</code><button type="button" onClick={() => void copy()} aria-label={label}>{state === "copied" ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}</button><span className="sr-only" aria-live="polite">{state === "copied" ? "Copied" : state === "failed" ? "Copy failed" : ""}</span></span>;
}

export function StatusBadge({ children, tone = "neutral" }: Readonly<{ children: ReactNode; tone?: "success" | "warning" | "danger" | "neutral" | "info" }>) {
  const icon = tone === "success" ? <CheckCircle weight="fill" aria-hidden="true" /> : tone === "warning" ? <WarningCircle weight="fill" aria-hidden="true" /> : tone === "danger" ? <XCircle weight="fill" aria-hidden="true" /> : <Info weight="fill" aria-hidden="true" />;
  return <span className={`status-label ${tone}`}>{icon}{children}</span>;
}

export function StatePanel({ title, message, kind = "empty", action }: Readonly<{ title: string; message: string; kind?: "empty" | "error" | "offline" | "denied" | "unknown"; action?: ReactNode }>) {
  return <div className={`state-panel ${kind}`} role={kind === "error" ? "alert" : "status"}>{kind === "error" || kind === "offline" || kind === "unknown" ? <WarningCircle weight="fill" aria-hidden="true" /> : <Info weight="fill" aria-hidden="true" />}<div><strong>{title}</strong><p>{message}</p>{action}</div></div>;
}

export function RecordLink({ href, label, id }: Readonly<{ href: string; label: string; id?: string }>) { return <Link className="record-link" href={href} id={id}>{label}<ArrowRight aria-hidden="true" /></Link>; }

export function Pagination({ nextCursor, onNext, busy, label = "Load more" }: Readonly<{ nextCursor?: string; onNext: () => void; busy?: boolean; label?: string }>) {
  return <div className="pagination"><span>{nextCursor ? "More records are available" : "End of available records"}</span><button className="button secondary" type="button" disabled={!nextCursor || busy} onClick={onNext}>{busy ? "Loading…" : label}</button></div>;
}

export function DataTableRegion({ label, children }: Readonly<{ label: string; children: ReactNode }>) {
  return <div className="data-table-wrap" role="region" aria-label={label} tabIndex={0}><p className="table-scroll-hint">Scroll horizontally to inspect every exact field.</p>{children}</div>;
}

export function PageHeader({ eyebrow, title, description, children }: Readonly<{ eyebrow: string; title: string; description: string; children?: ReactNode }>) {
  return <header className="page-header"><div><p className="eyebrow">{eyebrow}</p><h1>{title}</h1><p>{description}</p></div>{children}</header>;
}
