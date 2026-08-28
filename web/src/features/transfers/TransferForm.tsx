"use client";

import Link from "next/link";
import { type FormEvent, useEffect, useMemo, useRef, useState } from "react";

import type { Account } from "@/features/accounts/types";
import { hasPositiveMinorUnits } from "@/features/accounts/accountCommandIntent";
import { CopyControl, EvidenceFreshness, FocusedRetry, StatePanel } from "@/features/console/components";
import { accountLabel, utcDateTime } from "@/features/console/format";
import type { PreparedTransfer } from "@/features/transfers/transferIntent";
import { useTransferSubmission } from "@/features/transfers/useTransferSubmission";
import { decimalFromMinorUnits } from "@/lib/api/transfers";
import { formatMinorUnits, minorUnitsFromDecimal } from "@/lib/money";

type Props = Readonly<{
  accounts: Account[];
  accountsLoading: boolean;
  accountsError: string | null;
  accountsVerifiedAt?: string;
  tenantId: string;
  csrfToken: string;
  disabled?: boolean;
  disabledReason?: string;
  preferredDestinationId?: string;
  returnTo?: string;
  onRetryAccounts: () => void;
  onPosted: () => Promise<void>;
}>;

export function TransferForm({ accounts, accountsLoading, accountsError, accountsVerifiedAt, tenantId, csrfToken, disabled, disabledReason, preferredDestinationId, returnTo, onRetryAccounts, onPosted }: Props) {
  const transferable = useMemo(() => accounts.filter((account) => account.status === "active"), [accounts]);
  const fundedSources = useMemo(() => transferable.filter((account) => hasPositiveMinorUnits(account.available_minor)), [transferable]);
  const preferredDestination = useMemo(() => transferable.find((account) => account.account_id === preferredDestinationId), [preferredDestinationId, transferable]);
  const initialSource = preferredDestination
    ? fundedSources.find((account) => account.account_id !== preferredDestination.account_id && account.currency === preferredDestination.currency)
    : fundedSources[0];
  const [source, setSource] = useState(initialSource?.account_id ?? "");
  const [destination, setDestination] = useState(preferredDestination?.account_id ?? transferable.find((account) => account.account_id !== initialSource?.account_id)?.account_id ?? "");
  const [amount, setAmount] = useState("");
  const userChangedRoute = useRef(false);
  const preferredConsumed = useRef(false);
  const [prepared, setPrepared] = useState<PreparedTransfer | null>(null);
  const [validation, setValidation] = useState<string | null>(null);
  const { outcome, pending, setOutcome, storedIntent, submit: postPrepared } = useTransferSubmission(tenantId, csrfToken, onPosted);
  const reviewHeading = useRef<HTMLHeadingElement>(null);
  const outcomeHeading = useRef<HTMLHeadingElement>(null);

  const sourceAccount = fundedSources.find((account) => account.account_id === source) ?? fundedSources[0];
  const effectiveSource = sourceAccount?.account_id ?? "";
  const destinations = useMemo(
    () => transferable.filter((account) => account.account_id !== effectiveSource && account.currency === sourceAccount?.currency),
    [effectiveSource, sourceAccount?.currency, transferable],
  );
  const effectiveDestination = destinations.some((account) => account.account_id === destination)
    ? destination
    : destinations[0]?.account_id ?? "";

  const restorableIntent = useMemo(() => {
    if (!storedIntent) return null;
    const restoredSource = transferable.find((account) => account.account_id === storedIntent.sourceAccountId && account.currency === storedIntent.currency);
    const restoredDestination = transferable.find((account) => account.account_id === storedIntent.destinationAccountId && account.currency === storedIntent.currency);
    if (!restoredSource || !restoredDestination || restoredSource.account_id === restoredDestination.account_id) return null;
    return { source: restoredSource, destination: restoredDestination, amountMinor: storedIntent.amountMinor } satisfies PreparedTransfer;
  }, [storedIntent, transferable]);

  const effectivePrepared = prepared ?? restorableIntent;
  const preferredFundingBlocked = Boolean(preferredDestination && !storedIntent && !fundedSources.some((account) => account.account_id !== preferredDestination.account_id && account.currency === preferredDestination.currency));
  const pickerUnavailable = accountsLoading || Boolean(accountsError);
  const prerequisiteReason = accountsError
    ? "New transfer preparation is disabled because the authorized account picker could not be refreshed. Previously loaded accounts remain historical."
    : accountsLoading
      ? "New transfer preparation is disabled while the authorized account picker is loading."
      : disabledReason;

  useEffect(() => {
    if (preferredConsumed.current || userChangedRoute.current || storedIntent || !preferredDestination) return;
    const nextSource = fundedSources.find((account) => account.account_id !== preferredDestination.account_id && account.currency === preferredDestination.currency);
    if (!nextSource) return;
    const frame = requestAnimationFrame(() => {
      if (userChangedRoute.current || preferredConsumed.current) return;
      setSource(nextSource.account_id);
      setDestination(preferredDestination.account_id);
      preferredConsumed.current = true;
    });
    return () => cancelAnimationFrame(frame);
  }, [fundedSources, preferredDestination, storedIntent]);

  useEffect(() => {
    if (effectivePrepared) reviewHeading.current?.focus();
  }, [effectivePrepared]);

  useEffect(() => {
    if (outcome?.kind === "success") outcomeHeading.current?.focus();
  }, [outcome]);

  function prepare(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setValidation(null);
    setOutcome(null);
    const destinationAccount = destinations.find((account) => account.account_id === effectiveDestination);
    if (!sourceAccount || !destinationAccount || sourceAccount.account_id === destinationAccount.account_id) {
      setValidation("Choose two different active accounts in the same currency.");
      return;
    }
    try {
      setPrepared({
        source: sourceAccount,
        destination: destinationAccount,
        amountMinor: minorUnitsFromDecimal(sourceAccount.currency, amount),
      });
    } catch (error) {
      setValidation(error instanceof Error ? error.message : "Check the exact amount.");
    }
  }

  async function submit() {
    if (!effectivePrepared || pending) return;
    if (!prepared) {
      setPrepared(effectivePrepared);
      setSource(effectivePrepared.source.account_id);
      setDestination(effectivePrepared.destination.account_id);
      setAmount(decimalFromMinorUnits(effectivePrepared.source.currency, effectivePrepared.amountMinor));
    }
    if (await postPrepared(effectivePrepared)) {
      setPrepared(null);
      setAmount("");
    }
  }

  if (outcome?.kind === "success") {
    return <section className="surface transfer-outcome" aria-labelledby="transfer-outcome-heading">
      <p className="eyebrow">Final financial outcome</p>
      <h2 ref={outcomeHeading} tabIndex={-1} id="transfer-outcome-heading">Transfer posted</h2>
      <StatePanel title="Money moved exactly once" message={outcome.message} />
      <dl className="evidence-list">
        <div><dt>Transfer ID</dt><dd><Link href={`/transfers/${outcome.transferId}`}>{outcome.transferId}</Link></dd></div>
        <div><dt>Journal transaction</dt><dd>{outcome.journalTransactionId ? <CopyControl value={outcome.journalTransactionId} /> : <Link href={`/transfers/${outcome.transferId}`}>Open immutable record</Link>}</dd></div>
        <div><dt>Exact amount</dt><dd>{formatMinorUnits(outcome.currency!, outcome.amountMinor!)}</dd></div>
        <div><dt>Posted UTC</dt><dd>{outcome.occurredAt ? utcDateTime(outcome.occurredAt) : "Open the immutable record for timestamp evidence"}</dd></div>
        <div><dt>Source</dt><dd><Link href={`/accounts/${outcome.source}`}>{outcome.source}</Link></dd></div>
        <div><dt>Destination</dt><dd><Link href={`/accounts/${outcome.destination}`}>{outcome.destination}</Link></dd></div>
      </dl>
      <section className="committed-balance-evidence" aria-labelledby="committed-balances-heading">
        <p className="eyebrow" id="committed-balances-heading">Committed balance evidence</p>
        {outcome.balances?.length ? <div className="review-grid">{outcome.balances.map((balance) => <div key={balance.account_id}>
          <dt><Link href={`/accounts/${balance.account_id}`}>{balance.account_id === outcome.source ? "Source" : "Destination"} account</Link></dt>
          <dd>{formatMinorUnits(balance.currency, balance.posted_minor)}<code>version {balance.version}</code><small>{utcDateTime(balance.as_of)}</small></dd>
        </div>)}</div> : <p className="muted">Open the source and destination account records for current balance evidence.</p>}
      </section>
      <button className="button secondary" type="button" onClick={() => setOutcome(null)}>Prepare another transfer</button>
      {returnTo && <Link className="text-link" href={returnTo}>Return to account</Link>}
    </section>;
  }

  if (storedIntent && !restorableIntent) {
    return <StatePanel
      kind="denied"
      title="Unconfirmed transfer cannot be restored"
      message="The original accounts are no longer both available in the authorized active scope. LedgerSync will not alter or recreate this intent with a different key. Inspect transfer history before taking further action."
    />;
  }

  if (effectivePrepared) {
    const outcomeUnknown = outcome?.kind === "unknown";
    const submissionDisabled = Boolean(disabled || (!outcomeUnknown && pickerUnavailable));
    return <section className="surface transfer-review" aria-labelledby="transfer-review-heading">
      <p className="eyebrow">Review before posting</p>
      <h2 ref={reviewHeading} tabIndex={-1} id="transfer-review-heading">Confirm exact transfer</h2>
      <p>Verify source, destination, and exact amount together. Confirmation binds a retry-safe key to this complete intent.</p>
      <dl className="review-grid">
        <div><dt>From</dt><dd>{accountLabel(effectivePrepared.source)}<code>{effectivePrepared.source.account_id}</code></dd></div>
        <div><dt>To</dt><dd>{accountLabel(effectivePrepared.destination)}<code>{effectivePrepared.destination.account_id}</code></dd></div>
        <div><dt>Exact amount</dt><dd>{formatMinorUnits(effectivePrepared.source.currency, effectivePrepared.amountMinor)}</dd></div>
      </dl>
      {outcome && <StatePanel
        kind={outcomeUnknown ? "unknown" : "error"}
        title={outcomeUnknown ? "Result not yet confirmed" : "Transfer not posted"}
        message={outcome.message}
      />}
      {accountsError && <StatePanel kind="error" title="Account picker unavailable" message={accountsError} action={<FocusedRetry label="Retry account picker only" onRetry={onRetryAccounts} disabled={disabled} busy={accountsLoading} />} />}
      <div className="action-row">
        {outcomeUnknown
          ? <p className="intent-lock-note">Editing is locked until this exact outcome is confirmed.</p>
          : <button className="button secondary" type="button" disabled={pending} onClick={() => { setPrepared(null); setOutcome(null); }}>Back to edit</button>}
        <button className="button primary" type="button" aria-describedby={submissionDisabled ? "transfer-disabled-reason" : undefined} disabled={pending || submissionDisabled} onClick={() => void submit()}>
          {pending ? "Posting transfer…" : outcomeUnknown ? "Retry same transfer" : "Confirm and post"}
        </button>
      </div>
      {submissionDisabled && <p id="transfer-disabled-reason" className="permission-note">{prerequisiteReason ?? "Transfer posting is unavailable until connectivity and authorization are verified."}</p>}
    </section>;
  }

  if (accountsLoading && accounts.length === 0) {
    return <StatePanel title="Loading authorized account picker" message="Source and destination controls remain disabled until the account request succeeds. No empty account scope is inferred." />;
  }

  if (accountsError && accounts.length === 0) {
    return <StatePanel kind="error" title="Account picker unavailable" message={accountsError} action={<FocusedRetry label="Retry account picker only" onRetry={onRetryAccounts} disabled={disabled} busy={accountsLoading} />} />;
  }

  if (preferredFundingBlocked || transferable.length < 2 || fundedSources.length === 0 || destinations.length === 0) {
    return <StatePanel kind="denied" title="No funded source account" message="A different active, authorized account in the same currency must have a positive exact available balance before a new transfer can be prepared." />;
  }

  return <section className="surface transfer-form" aria-labelledby="transfer-heading">
    <div>
      <p className="eyebrow">Prepare</p>
      <h2 id="transfer-heading">Internal transfer</h2>
      <p className="muted">Exact, same-currency movement between authorized ledger accounts.</p>
    </div>
    {accountsVerifiedAt && <EvidenceFreshness state={accountsError ? "historical" : accountsLoading ? "refreshing" : "current"} verifiedAt={accountsVerifiedAt} label="Account picker" reason={accountsError ?? undefined} />}
    {accountsError && <StatePanel kind="error" title="Account picker not refreshed" message={accountsError} action={<FocusedRetry label="Retry account picker only" onRetry={onRetryAccounts} disabled={disabled} busy={accountsLoading} />} />}
    <form onSubmit={prepare} noValidate>
      <label>From account<select value={effectiveSource} onChange={(event) => { userChangedRoute.current = true; setSource(event.target.value); }} disabled={disabled || pickerUnavailable}>{fundedSources.map((account) => <option key={account.account_id} value={account.account_id}>{accountLabel(account)} · {account.currency}</option>)}</select></label>
      <label>To account<select value={effectiveDestination} onChange={(event) => { userChangedRoute.current = true; setDestination(event.target.value); }} disabled={disabled || pickerUnavailable}>{destinations.map((account) => <option key={account.account_id} value={account.account_id}>{accountLabel(account)} · {account.currency}</option>)}</select></label>
      <label>Exact amount<input value={amount} onChange={(event) => setAmount(event.target.value)} inputMode="decimal" autoComplete="off" placeholder="0.00" aria-describedby="transfer-help transfer-error transfer-disabled-reason" aria-invalid={Boolean(validation)} disabled={disabled || pickerUnavailable} /></label>
      <p id="transfer-help" className="muted">Decimal text becomes integer minor units. Floating-point arithmetic is never used.</p>
      {preferredDestination && !storedIntent && <p className="destination-preselection" role="status">Destination preselected from account <code>{preferredDestination.account_id}</code>. Review the source and exact amount before posting.</p>}
      {validation && <p id="transfer-error" className="field-error" role="alert">{validation}</p>}
      <button className="button primary" disabled={disabled || pickerUnavailable} aria-describedby={disabled || pickerUnavailable ? "transfer-disabled-reason" : undefined} type="submit">Review transfer</button>
      {(disabled || pickerUnavailable) && <p id="transfer-disabled-reason" className="permission-note">{prerequisiteReason ?? "Transfer preparation is disabled until connectivity and authorization are verified."}</p>}
    </form>
  </section>;
}
