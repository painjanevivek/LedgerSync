import { ArrowRight, Wallet } from "@phosphor-icons/react";
import { accountLabel } from "@/features/console/format";
import { Money } from "@/ui/display/Money";
import { Timestamp } from "@/ui/display/Timestamp";
import { TechnicalDetails } from "@/ui/presentation/TechnicalDetails";
import { RecordIdentity } from "@/ui/presentation/RecordIdentity";
import { RelativeTime } from "@/ui/presentation/RelativeTime";
import type { PreparedTransfer } from "./transferIntent";
import { expectedTransferBalances } from "./transferReviewModel";

export function TransferReview({ transfer, unresolved = false }: Readonly<{ transfer: PreparedTransfer; unresolved?: boolean }>) {
  // An unresolved request may already have moved the money. Do not project a second movement.
  let expected: ReturnType<typeof expectedTransferBalances> | null = null;
  if (!unresolved) { try { expected = expectedTransferBalances(transfer); } catch { /* The controller exposes the blocking validation. */ } }
  return <div className="guided-transfer-review">
    <p>You’re transferring</p><strong className="review-money"><Money localized currency={transfer.source.currency} minorUnits={transfer.amountMinor} /></strong>
    <div className="review-account-route"><div><Wallet aria-hidden="true" /><span>From<strong>{accountLabel(transfer.source)}</strong></span></div><ArrowRight aria-hidden="true" /><div><Wallet aria-hidden="true" /><span>To<strong>{accountLabel(transfer.destination)}</strong></span></div></div>
    {expected && <section className="expected-effects" aria-labelledby="expected-balances">
      <h3 id="expected-balances">Expected balances after this transfer</h3>
      <dl>{[
        { account: transfer.source, after: expected.source },
        { account: transfer.destination, after: expected.destination },
      ].map(({ account, after }) => <div key={account.account_id}>
        <dt>{accountLabel(account)}<small><RelativeTime value={account.as_of} /></small></dt>
        <dd><span><span className="sr-only">Before: </span><Money localized currency={account.currency} minorUnits={account.available_minor} /></span><ArrowRight aria-hidden="true" /><span><span className="sr-only">Expected after: </span><Money localized currency={account.currency} minorUnits={after} /></span></dd>
      </div>)}</dl>
      <p>These are expected available balances, not a completed result. We’ll check the accounts again before submitting.</p>
    </section>}
    <TechnicalDetails summary="View account and balance details"><RecordIdentity label="Source account" value={transfer.source.account_id} /><RecordIdentity label="Destination account" value={transfer.destination.account_id} /><p>Source balance time: <Timestamp value={transfer.source.as_of} /></p><p>Destination balance time: <Timestamp value={transfer.destination.as_of} /></p></TechnicalDetails>
  </div>;
}
