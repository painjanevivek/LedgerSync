"use client";

import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";

import type { Account } from "@/features/accounts/types";
import { StatePanel } from "@/features/console/components";
import { accountLabel } from "@/features/console/format";
import { type PreparedTransfer, useTransferSubmission } from "@/features/transfers/useTransferSubmission";
import { formatMinorUnits, minorUnitsFromDecimal } from "@/lib/money";

type Props = Readonly<{ accounts: Account[]; tenantId: string; csrfToken: string; disabled?: boolean; onPosted: () => Promise<void> }>;

export function TransferForm({ accounts, tenantId, csrfToken, disabled, onPosted }: Props) {
  const transferable = useMemo(() => accounts.filter((account) => account.status === "active"), [accounts]);
  const [source, setSource] = useState(transferable[0]?.account_id ?? ""); const [destination, setDestination] = useState(transferable[1]?.account_id ?? ""); const [amount, setAmount] = useState("");
  const [prepared, setPrepared] = useState<PreparedTransfer | null>(null); const [validation, setValidation] = useState<string | null>(null);
  const { outcome, pending, setOutcome, submit: postPrepared } = useTransferSubmission(tenantId, csrfToken, onPosted);
  const reviewHeading = useRef<HTMLHeadingElement>(null); const outcomeHeading = useRef<HTMLHeadingElement>(null);
  const sourceAccount = transferable.find((account) => account.account_id === source) ?? transferable[0];
  const effectiveSource = sourceAccount?.account_id ?? "";
  const destinations = useMemo(() => transferable.filter((account) => account.account_id !== effectiveSource && account.currency === sourceAccount?.currency), [effectiveSource, sourceAccount?.currency, transferable]);
  const effectiveDestination = destinations.some((account) => account.account_id === destination) ? destination : destinations[0]?.account_id ?? "";
  useEffect(() => { if (prepared) reviewHeading.current?.focus(); }, [prepared]); useEffect(() => { if (outcome) outcomeHeading.current?.focus(); }, [outcome]);

  function prepare(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setValidation(null); setOutcome(null);
    const destinationAccount = destinations.find((account) => account.account_id === effectiveDestination);
    if (!sourceAccount || !destinationAccount || sourceAccount.account_id === destinationAccount.account_id) { setValidation("Choose two different active accounts in the same currency."); return; }
    try { setPrepared({ source: sourceAccount, destination: destinationAccount, amountMinor: minorUnitsFromDecimal(sourceAccount.currency, amount) }); }
    catch (error) { setValidation(error instanceof Error ? error.message : "Check the exact amount."); }
  }

  async function submit() {
    if (!prepared || pending) return;
    if (await postPrepared(prepared)) { setPrepared(null); setAmount(""); }
  }

  if (transferable.length < 2 || destinations.length === 0) return <StatePanel kind="denied" title="Transfer unavailable" message="Two active, authorized accounts in the same currency are required." />;
  if (outcome?.kind === "success") return <section className="surface transfer-outcome" aria-labelledby="transfer-outcome-heading"><p className="eyebrow">Final financial outcome</p><h2 ref={outcomeHeading} tabIndex={-1} id="transfer-outcome-heading">Transfer posted</h2><StatePanel title="Money moved exactly once" message={outcome.message} /><dl className="evidence-list"><div><dt>Transfer ID</dt><dd><Link href={`/transfers/${outcome.transferId}`}>{outcome.transferId}</Link></dd></div><div><dt>Exact amount</dt><dd>{formatMinorUnits(outcome.currency!, outcome.amountMinor!)}</dd></div><div><dt>Source</dt><dd><Link href={`/accounts/${outcome.source}`}>{outcome.source}</Link></dd></div><div><dt>Destination</dt><dd><Link href={`/accounts/${outcome.destination}`}>{outcome.destination}</Link></dd></div></dl><button className="button secondary" type="button" onClick={() => setOutcome(null)}>Prepare another transfer</button></section>;
  if (prepared) return <section className="surface transfer-review" aria-labelledby="transfer-review-heading"><p className="eyebrow">Review before posting</p><h2 ref={reviewHeading} tabIndex={-1} id="transfer-review-heading">Confirm exact transfer</h2><p>Verify all three financial facts together. Confirmation creates the retry-safe request key.</p><dl className="review-grid"><div><dt>From</dt><dd>{accountLabel(prepared.source)}<code>{prepared.source.account_id}</code></dd></div><div><dt>To</dt><dd>{accountLabel(prepared.destination)}<code>{prepared.destination.account_id}</code></dd></div><div><dt>Exact amount</dt><dd>{formatMinorUnits(prepared.source.currency, prepared.amountMinor)}</dd></div></dl>{outcome && <StatePanel kind={outcome.kind === "unknown" ? "unknown" : "error"} title={outcome.kind === "unknown" ? "Result not yet confirmed" : "Transfer not posted"} message={outcome.message} />}<div className="action-row"><button className="button secondary" type="button" disabled={pending} onClick={() => { setPrepared(null); setOutcome(null); }}>Back to edit</button><button className="button primary" type="button" disabled={pending || disabled} onClick={() => void submit()}>{pending ? "Posting transfer…" : outcome?.kind === "unknown" ? "Retry same transfer" : "Confirm and post"}</button></div></section>;
  return <section className="surface transfer-form" aria-labelledby="transfer-heading"><div><p className="eyebrow">Prepare</p><h2 id="transfer-heading">Internal transfer</h2><p className="muted">Exact, same-currency movement between authorized ledger accounts.</p></div><form onSubmit={prepare} noValidate><label>From account<select value={effectiveSource} onChange={(event) => setSource(event.target.value)} disabled={disabled}>{transferable.map((account) => <option key={account.account_id} value={account.account_id}>{accountLabel(account)} · {account.currency}</option>)}</select></label><label>To account<select value={effectiveDestination} onChange={(event) => setDestination(event.target.value)} disabled={disabled}>{destinations.map((account) => <option key={account.account_id} value={account.account_id}>{accountLabel(account)} · {account.currency}</option>)}</select></label><label>Exact amount<input value={amount} onChange={(event) => setAmount(event.target.value)} inputMode="decimal" autoComplete="off" placeholder="0.00" aria-describedby="transfer-help transfer-error" aria-invalid={Boolean(validation)} disabled={disabled}/></label><p id="transfer-help" className="muted">Decimal text becomes integer minor units. Floating-point arithmetic is never used.</p>{validation && <p id="transfer-error" className="field-error" role="alert">{validation}</p>}<button className="button primary" disabled={disabled} type="submit">Review transfer</button></form></section>;
}
