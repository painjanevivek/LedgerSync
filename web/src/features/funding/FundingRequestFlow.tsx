"use client";

import { ArrowLeft, CheckCircle, Receipt, WarningCircle, X } from "@phosphor-icons/react";
import { FormEvent, useMemo, useState } from "react";

import type { Account } from "@/features/accounts/types";
import { accountLabel } from "@/features/console/format";
import type { FundingEvent, FundingSubmission } from "@/lib/api/funding";
import { minorUnitsFromDecimal, formatMinorUnits } from "@/lib/money";

type Props = Readonly<{
  accounts: Account[];
  csrfToken: string;
  online: boolean;
  canWrite: boolean;
  open: boolean;
  onClose: () => void;
  onCreated: (event: FundingEvent) => Promise<void>;
}>;

type PreparedEvidence = Readonly<{
  destinationAccountId: string;
  amountMinor: string;
  currency: string;
  externalReference: string;
  evidenceReference: string;
  idempotencyKey: string;
}>;

export function FundingRequestFlow({ accounts, csrfToken, online, canWrite, open, onClose, onCreated }: Props) {
  const eligible = useMemo(() => accounts.filter((account) => account.status === "active"), [accounts]);
  const [destinationAccountId, setDestinationAccountId] = useState("");
  const [amount, setAmount] = useState("");
  const [externalReference, setExternalReference] = useState("");
  const [evidenceReference, setEvidenceReference] = useState("");
  const [prepared, setPrepared] = useState<PreparedEvidence | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const destination = eligible.find((account) => account.account_id === destinationAccountId);

  function prepare(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    try {
      if (!destination) throw new Error("Choose an active destination account.");
      const amountMinor = minorUnitsFromDecimal(destination.currency, amount);
      if (amountMinor === "0") throw new Error("Enter an amount greater than zero.");
      if (!externalReference.trim() || !evidenceReference.trim()) throw new Error("Record both the external reference and its evidence location.");
      setPrepared({ destinationAccountId, amountMinor, currency: destination.currency, externalReference: externalReference.trim(), evidenceReference: evidenceReference.trim(), idempotencyKey: crypto.randomUUID() });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "The evidence request is invalid.");
    }
  }

  async function recordEvidence() {
    if (!prepared) return;
    setBusy(true);
    setError(null);
    try {
      const response = await fetch("/api/funding-requests", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "Idempotency-Key": prepared.idempotencyKey },
        body: JSON.stringify({ destinationAccountId: prepared.destinationAccountId, amountMinor: prepared.amountMinor, currency: prepared.currency, externalReference: prepared.externalReference, evidenceReference: prepared.evidenceReference }),
      });
      const payload = await response.json() as FundingSubmission & { error?: string };
      if (!response.ok || !payload.event?.funding_event_id) throw new Error(response.status === 504 ? "Outcome unknown. Retry this exact review with the same evidence; LedgerSync will reuse its idempotency key." : `Evidence was not recorded (${payload.error ?? response.status}).`);
      await onCreated(payload.event);
      setPrepared(null);
      setDestinationAccountId(""); setAmount(""); setExternalReference(""); setEvidenceReference("");
      onClose();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Evidence could not be recorded.");
    } finally {
      setBusy(false);
    }
  }

  if (!open) return null;
  return <section className="funding-request-panel" aria-labelledby="funding-request-heading">
    <header><div className="funding-panel-mark"><Receipt weight="fill" aria-hidden="true" /></div><div><p className="eyebrow">Controlled intake</p><h2 id="funding-request-heading">Record external value</h2><p>This creates a reviewable funding record. It does not claim a bank deposit or settled custody.</p></div><button className="icon-button funding-close" type="button" aria-label="Close funding request" onClick={onClose}><X aria-hidden="true" /></button></header>
    {!canWrite ? <div className="funding-inline-notice"><WarningCircle weight="fill" aria-hidden="true" /><p><strong>Write scope required.</strong> Ask a tenant administrator for funding:write before recording funding.</p></div> : !prepared ? <form className="funding-evidence-form" onSubmit={prepare}>
      <label>Destination account<select required value={destinationAccountId} onChange={(event) => setDestinationAccountId(event.target.value)}><option value="">Choose an active account</option>{eligible.map((account) => <option key={account.account_id} value={account.account_id}>{accountLabel(account)} · {account.currency}</option>)}</select></label>
      <label>Exact amount<input required inputMode="decimal" autoComplete="off" value={amount} onChange={(event) => setAmount(event.target.value)} placeholder="1250.00" /></label>
      <label>External reference<input required maxLength={256} value={externalReference} onChange={(event) => setExternalReference(event.target.value)} placeholder="Provider or bank reference" /></label>
      <label>Evidence location<input required maxLength={512} value={evidenceReference} onChange={(event) => setEvidenceReference(event.target.value)} placeholder="Controlled document or case reference" /></label>
      <div className="funding-form-note"><CheckCircle weight="fill" aria-hidden="true" /><p>LedgerSync stores exact minor units and requires a separate production finance operator to approve this record.</p></div>
      {error && <p className="form-error" role="alert">{error}</p>}
      <footer><button className="button secondary" type="button" onClick={onClose}>Cancel</button><button className="button primary" type="submit" disabled={!online}>Review funding</button></footer>
    </form> : <div className="funding-review">
      <div className="review-kicker"><span>Evidence review</span><strong>No journal posting yet</strong></div>
      <dl><div><dt>Destination</dt><dd>{destination ? accountLabel(destination) : prepared.destinationAccountId}</dd></div><div><dt>Exact amount</dt><dd className="number-cell">{formatMinorUnits(prepared.currency, prepared.amountMinor)}</dd></div><div><dt>External reference</dt><dd>{prepared.externalReference}</dd></div><div><dt>Supporting reference</dt><dd>{prepared.evidenceReference}</dd></div></dl>
      <div className="funding-inline-notice"><WarningCircle weight="fill" aria-hidden="true" /><p><strong>Confirm the claim, not settlement.</strong> Recording creates an immutable request for approval; balances change only after an approved journal is posted.</p></div>
      {error && <p className="form-error" role="alert">{error}</p>}
      <footer><button className="button secondary" type="button" disabled={busy} onClick={() => setPrepared(null)}><ArrowLeft aria-hidden="true" />Edit funding</button><button className="button primary" type="button" disabled={busy || !online} onClick={() => void recordEvidence()}>{busy ? "Recording…" : "Record for review"}</button></footer>
    </div>}
  </section>;
}
