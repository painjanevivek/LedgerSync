"use client";

import { formatMinorUnits } from "@/lib/money";

type Props = Readonly<{ currency?: string; availableMinor?: string; version?: string; asOf?: string; loading?: boolean; error?: string | null }>;

export function BalanceStatus({ currency, availableMinor, version, asOf, loading, error }: Props) {
  if (loading) return <section className="balance-status surface" aria-busy="true"><p className="eyebrow">Posted balance</p><div className="skeleton amount-skeleton" /><p className="muted">Checking the current authoritative balance…</p></section>;
  if (error || !currency || availableMinor === undefined) return <section className="balance-status surface state-unavailable" role="status"><p className="eyebrow">Posted balance</p><strong>Temporarily unavailable</strong><p>{error ?? "We will not show an older value as current."}</p></section>;
  return <section className="balance-status surface"><p className="eyebrow">Posted balance</p><strong className="amount-xl" aria-label={`${formatMinorUnits(currency, availableMinor)} posted balance`}>{formatMinorUnits(currency, availableMinor)}</strong><p className="muted">Version <span className="mono">{version ?? "—"}</span> · As of {asOf ? new Date(asOf).toLocaleString() : "—"}</p></section>;
}
