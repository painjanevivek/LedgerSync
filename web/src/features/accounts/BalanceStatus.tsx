"use client";

import { EvidenceFreshness } from "@/features/console/components";
import { formatMinorUnits } from "@/lib/money";

type Props = Readonly<{ currency?: string; availableMinor?: string; version?: string; asOf?: string; verifiedAt?: string; loading?: boolean; error?: string | null }>;

export function BalanceStatus({ currency, availableMinor, version, asOf, verifiedAt, loading, error }: Props) {
  const hasEvidence = Boolean(currency && availableMinor !== undefined);
  if (loading && !hasEvidence) return <section className="balance-status surface" aria-busy="true"><p className="eyebrow">Posted balance</p><div className="skeleton amount-skeleton" aria-hidden="true" /><p className="muted">Checking the current authoritative balance…</p></section>;
  if (!hasEvidence) return <section className="balance-status surface state-unavailable" role="status"><p className="eyebrow">Posted balance</p><strong>Temporarily unavailable</strong><p>{error ?? "No older value is presented as current."}</p></section>;
  const historical = Boolean(error);
  return <section className={`balance-status surface${historical ? " state-unavailable" : ""}`} aria-busy={loading}>
    <p className="eyebrow">{historical ? "Last verified posted balance" : "Posted balance"}</p>
    <strong className="amount-xl" aria-label={`${formatMinorUnits(currency!, availableMinor!)} ${historical ? "historical posted balance" : "posted balance"}`}>{formatMinorUnits(currency!, availableMinor!)}</strong>
    <p className="muted">Version <span className="mono">{version ?? "—"}</span> · As of {asOf ? new Date(asOf).toLocaleString() : "—"}</p>
    {verifiedAt && <EvidenceFreshness state={historical ? "historical" : loading ? "refreshing" : "current"} verifiedAt={verifiedAt} label="Balance evidence" reason={historical ? error ?? undefined : undefined} />}
  </section>;
}
