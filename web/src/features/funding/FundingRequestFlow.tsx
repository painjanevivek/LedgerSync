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
      if (!destination) throw new Error("Choose the account that should receive this amount.");
      const amountMinor = minorUnitsFromDecimal(destination.currency, amount);
      if (amountMinor === "0") throw new Error("Enter an amount greater than zero.");
      if (!externalReference.trim() || !evidenceReference.trim()) throw new Error("Add both the reference number and the supporting document location.");
      setPrepared({ destinationAccountId, amountMinor, currency: destination.currency, externalReference: externalReference.trim(), evidenceReference: evidenceReference.trim(), idempotencyKey: crypto.randomUUID() });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Check the required fields and try again.");
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
      if (!response.ok || !payload.event?.funding_event_id) throw new Error(response.status === 504 ? "We could not confirm whether the record was saved. Retry this same review safely; LedgerSync will not create a duplicate." : `The record was not saved (${payload.error ?? response.status}).`);
      await onCreated(payload.event);
      setPrepared(null);
      setDestinationAccountId(""); setAmount(""); setExternalReference(""); setEvidenceReference("");
      onClose();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "The record could not be saved.");
    } finally {
      setBusy(false);
    }
  }

  if (!open) return null;
  return <section className="funding-request-panel" aria-labelledby="funding-request-heading">
    <header><div className="funding-panel-mark"><Receipt weight="fill" aria-hidden="true" /></div><div><p className="eyebrow">Step 1 of 2</p><h2 id="funding-request-heading">Add a funding record</h2><p>Add the four details needed for review. This creates an external value reference; it does not claim that LedgerSync holds the money or that a bank transfer has settled.</p></div><button className="icon-button funding-close" type="button" aria-label="Close funding request" onClick={onClose}><X aria-hidden="true" /></button></header>
    {!canWrite ? <div className="funding-inline-notice"><WarningCircle weight="fill" aria-hidden="true" /><p><strong>Write scope required.</strong> Ask a tenant administrator for funding:write before recording funding.</p></div> : !prepared ? <form className="funding-evidence-form" onSubmit={prepare}>
      <div className="funding-field"><label htmlFor="funding-destination-account">Account <span className="required-badge" aria-hidden="true">Required</span></label><p id="funding-destination-account-help">Choose the account that should receive this amount.</p><select id="funding-destination-account" aria-describedby="funding-destination-account-help" required value={destinationAccountId} onChange={(event) => setDestinationAccountId(event.target.value)}><option value="">Choose an account</option>{eligible.map((account) => <option key={account.account_id} value={account.account_id}>{accountLabel(account)} · {account.currency}</option>)}</select></div>
      <div className="funding-field"><label htmlFor="funding-amount">Amount <span className="required-badge" aria-hidden="true">Required</span></label><p id="funding-amount-help">Enter the amount in INR. Example: 1250.00</p><input id="funding-amount" aria-describedby="funding-amount-help" required inputMode="decimal" autoComplete="off" value={amount} onChange={(event) => setAmount(event.target.value)} placeholder="1250.00" /></div>
      <div className="funding-field"><label htmlFor="funding-reference">Reference number <span className="required-badge" aria-hidden="true">Required</span></label><p id="funding-reference-help">Use the number from your bank, provider, or payment record.</p><input id="funding-reference" aria-describedby="funding-reference-help" required maxLength={256} value={externalReference} onChange={(event) => setExternalReference(event.target.value)} placeholder="Example: BANK-REF-1234" /></div>
      <div className="funding-field"><label htmlFor="funding-supporting-document">Supporting document <span className="required-badge" aria-hidden="true">Required</span></label><p id="funding-supporting-document-help">Add the case, receipt, or document ID where this can be checked.</p><input id="funding-supporting-document" aria-describedby="funding-supporting-document-help" required maxLength={512} value={evidenceReference} onChange={(event) => setEvidenceReference(event.target.value)} placeholder="Example: CASE-104 or receipt ID" /></div>
      <div className="funding-form-note"><CheckCircle weight="fill" aria-hidden="true" /><p><strong>Why all four?</strong> Another operator needs them to check the record before your balance can change.</p></div>
      {error && <p className="form-error" role="alert">{error}</p>}
      <footer><button className="button secondary" type="button" onClick={onClose}>Cancel</button><button className="button primary" type="submit" disabled={!online}>Review details</button></footer>
    </form> : <div className="funding-review">
      <div className="review-kicker"><span>Check your details</span><strong>Your balance will not change yet</strong></div>
      <dl><div><dt>Account</dt><dd>{destination ? accountLabel(destination) : prepared.destinationAccountId}</dd></div><div><dt>Amount</dt><dd className="number-cell">{formatMinorUnits(prepared.currency, prepared.amountMinor)}</dd></div><div><dt>Reference number</dt><dd>{prepared.externalReference}</dd></div><div><dt>Supporting document</dt><dd>{prepared.evidenceReference}</dd></div></dl>
      <div className="funding-inline-notice"><WarningCircle weight="fill" aria-hidden="true" /><p><strong>What happens next?</strong> Saving sends this record for review. Your balance changes only after another operator approves it.</p></div>
      {error && <p className="form-error" role="alert">{error}</p>}
      <footer><button className="button secondary" type="button" disabled={busy} onClick={() => setPrepared(null)}><ArrowLeft aria-hidden="true" />Edit details</button><button className="button primary" type="button" disabled={busy || !online} onClick={() => void recordEvidence()}>{busy ? "Saving…" : "Save for review"}</button></footer>
    </div>}
  </section>;
}
