"use client";
import { FocusedRetry } from "@/ui/controls/FocusedRetry.client";
import { EvidenceFreshness } from "@/ui/display/Evidence";
import { Money } from "@/ui/display/Money";
import { Timestamp } from "@/ui/display/Timestamp";
import { reservedMinorUnits, formatMinorUnits } from "@/lib/money";
type Props = Readonly<{
    currency?: string;
    availableMinor?: string;
    ledgerMinor?: string;
    version?: string;
    asOf?: string;
    verifiedAt?: string;
    loading?: boolean;
    error?: string | null;
    onRetry?: () => void;
    retryDisabled?: boolean;
}>;
export function BalanceStatus({ currency, availableMinor, ledgerMinor, version, asOf, verifiedAt, loading, error, onRetry, retryDisabled }: Props) {
    const hasEvidence = Boolean(currency && availableMinor !== undefined);
    if (loading && !hasEvidence)
        return <section className="balance-status surface" aria-busy="true"><p className="eyebrow">Available balance</p><div className="skeleton amount-skeleton" aria-hidden="true"/><p className="muted">Checking the current authoritative balance…</p></section>;
    if (!hasEvidence)
        return <section className="balance-status surface state-unavailable" role="status"><p className="eyebrow">Available balance</p><strong>Temporarily unavailable</strong><p>{error ?? "No older value is presented as current."}</p>{error && onRetry && <FocusedRetry label="Retry balance only" onRetry={onRetry} disabled={retryDisabled} busy={loading}/>}</section>;
    const historical = Boolean(error);
    const reserved = ledgerMinor === undefined ? null : reservedMinorUnits(ledgerMinor, availableMinor!);
    return <section className={`balance-status surface${historical ? " state-unavailable" : ""}`} aria-busy={loading}>
    <p className="eyebrow">{historical ? "Last verified available balance" : "Available balance"}</p>
    <strong className="amount-xl" aria-label={`${formatMinorUnits(currency!, availableMinor!)} ${historical ? "historical available balance" : "available balance"}`}><Money currency={currency!} minorUnits={availableMinor!}/></strong>
    {ledgerMinor !== undefined && <dl className="review-grid balance-components"><div><dt>Posted ledger</dt><dd><Money currency={currency!} minorUnits={ledgerMinor}/></dd></div><div><dt>Reserved</dt><dd>{reserved === null ? "Unavailable" : <Money currency={currency!} minorUnits={reserved}/>}</dd></div></dl>}
    <p className="muted">Version <span className="mono">{version ?? "—"}</span> · As of <Timestamp value={asOf} fallback="—"/></p>
    {verifiedAt && <EvidenceFreshness state={historical ? "historical" : loading ? "refreshing" : "current"} verifiedAt={verifiedAt} label="Balance details" reason={historical ? error ?? undefined : undefined}/>}
    {error && onRetry && <FocusedRetry label="Retry balance only" onRetry={onRetry} disabled={retryDisabled} busy={loading}/>}
  </section>;
}
