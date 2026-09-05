import Link from "next/link";
import type { RefObject } from "react";
import { Money } from "@/ui/display/Money";
import { Timestamp } from "@/ui/display/Timestamp";
import { TechnicalDetails } from "@/ui/presentation/TechnicalDetails";
import { RecordIdentity } from "@/ui/presentation/RecordIdentity";
import type { TransferOutcome } from "./useTransferSubmission";

export function TransferResult({ outcome, headingRef, onAnother, canStartAnother }: Readonly<{ outcome: NonNullable<TransferOutcome>; headingRef: RefObject<HTMLHeadingElement | null>; onAnother: () => void; canStartAnother: boolean }>) {
  return <section className="transfer-outcome" aria-labelledby="transfer-outcome-heading"><h2 id="transfer-outcome-heading" ref={headingRef} tabIndex={-1}>Transfer completed</h2><p role="status">The money moved between your accounts. This result is confirmed.</p><strong className="review-money"><Money localized currency={outcome.currency!} minorUnits={outcome.amountMinor!} /></strong>
    <TechnicalDetails summary="View transfer details"><RecordIdentity label="Transfer reference" value={outcome.transferId!} />{outcome.journalTransactionId && <RecordIdentity label="Journal reference" value={outcome.journalTransactionId} />}{outcome.requestReference && <RecordIdentity label="Request reference" value={outcome.requestReference} />}<p>Completed: <Timestamp value={outcome.occurredAt} /></p><RecordIdentity label="Source account" value={outcome.source!} /><RecordIdentity label="Destination account" value={outcome.destination!} /><h3>Confirmed balance details</h3>{outcome.balances?.length ? <dl className="evidence-list">{outcome.balances.map(balance => <div key={balance.account_id}><dt>{balance.account_id === outcome.source ? "Source" : "Destination"}</dt><dd><Money localized currency={balance.currency} minorUnits={balance.posted_minor} /><p>Version {balance.version} · <Timestamp value={balance.as_of} /></p></dd></div>)}</dl> : <p>Open the account records to check their latest balances.</p>}</TechnicalDetails>
    <div className="command-actions"><Link className="button primary" href={`/transfers/${encodeURIComponent(outcome.transferId!)}`}>View transfer</Link>{canStartAnother ? <button type="button" className="button secondary" onClick={onAnother}>Make another transfer</button> : <p role="alert">This transfer completed, but its local retry information could not be cleared. Do not create another request in this browser until this is resolved.</p>}</div>
  </section>;
}
