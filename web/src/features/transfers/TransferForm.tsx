"use client";

import Link from "next/link";
import { type ReactNode, type FormEvent, useEffect, useMemo, useRef, useState } from "react";

import type { Account } from "@/features/accounts/types";
import { hasPositiveMinorUnits } from "@/features/accounts/accountCommandIntent";

import { FocusedRetry } from "@/ui/controls/FocusedRetry.client";
import { EvidenceFreshness } from "@/ui/display/Evidence";
import { StatePanel } from "@/ui/display/StatePanel";
import { FormField } from "@/ui/forms/FormField.client";
import { accountLabel } from "@/features/console/format";
import type { PreparedTransfer } from "@/features/transfers/transferIntent";
import { useTransferSubmission } from "@/features/transfers/useTransferSubmission";
import { decimalFromMinorUnits } from "@/lib/api/transfers";
import { minorUnitsFromDecimal } from "@/lib/money";


import { ActionAvailability, type ActionAvailabilityStatus } from "@/ui/controls/ActionAvailability";

import { CommandFrame } from "@/ui/presentation/CommandFrame";
import { TransferReview } from "./TransferReview";
import { TransferResult } from "./TransferResult";
import { expectedTransferBalances, refreshTransferReview, reviewAccountChanged } from "./transferReviewModel";
import { TechnicalDetails } from "@/ui/presentation/TechnicalDetails";
import { RecordIdentity } from "@/ui/presentation/RecordIdentity";

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
  const { outcome, pending, setOutcome, storedIntent, storageBlocked, submit: postPrepared } = useTransferSubmission(tenantId, csrfToken, onPosted);
  const reviewHeading = useRef<HTMLHeadingElement>(null);
  const outcomeHeading = useRef<HTMLHeadingElement>(null);
  const [checking, setChecking] = useState(false);
  const preflight = useRef<AbortController | null>(null);
  useEffect(() => () => preflight.current?.abort(), []);
  const busy = pending || checking;
  const dirty = Boolean(amount || prepared);
  useEffect(() => {
    if (!dirty || storedIntent || outcome?.kind === "success") return;
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ""; };
    const guardLink = (event: MouseEvent) => {
      const anchor = (event.target as Element).closest?.("a[href]") as HTMLAnchorElement | null;
      if (!anchor || anchor.target === "_blank" || anchor.origin !== location.origin || anchor.href === location.href || event.defaultPrevented) return;
      if (!window.confirm("Leave this transfer? Your edited, unsubmitted details will be discarded.")) { event.preventDefault(); event.stopPropagation(); }
    };
    window.addEventListener("beforeunload", warn);
    document.addEventListener("click", guardLink, true);
    return () => { window.removeEventListener("beforeunload", warn); document.removeEventListener("click", guardLink, true); };
  }, [dirty, storedIntent, outcome?.kind]);
  function frame(content: ReactNode, stage: "details" | "review" | "result") {
    return <CommandFrame title={stage === "details" ? "Make a transfer." : stage === "review" ? "Check before you transfer." : "Your transfer result."} description={stage === "details" ? "Move money between your accounts in three clear steps." : stage === "review" ? "Review the amount and accounts. Nothing moves until you confirm." : "See what happened and your safe next step."} stage={stage} returnTo={returnTo ?? "/transfers"} returnLabel={returnTo?.startsWith("/accounts") ? "Back to account" : "Back to transfers"} help={<ul><li>Check the source and destination accounts.</li><li>Keep enough money in your source account.</li><li>An uncertain result must be resolved before making another request.</li></ul>}>{storageBlocked && <StatePanel kind="error" title="Transfer retry information is unavailable" message="No new request can be submitted. Allow browser storage, then reload. Existing request information has not been overwritten." />}{content}</CommandFrame>;
  }

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
  const preparationAvailability: ActionAvailabilityStatus = accountsLoading
    ? { state: "busy", reason: "Wait while your available accounts are checked." }
    : accountsError
      ? { state: "temporary_unavailable", reason: prerequisiteReason ?? "The account picker is unavailable." }
      : disabled
        ? { state: disabledReason?.toLowerCase().includes("offline") ? "offline" : "capability_missing", reason: prerequisiteReason ?? "You cannot prepare a transfer right now." }
        : { state: "available" };

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
      const next = {
        source: sourceAccount,
        destination: destinationAccount,
        amountMinor: minorUnitsFromDecimal(sourceAccount.currency, amount),
      };
      expectedTransferBalances(next);
      setPrepared(next);
    } catch (error) {
      setValidation(error instanceof Error ? error.message : "Check the exact amount.");
    }
  }

  async function submit() {
    if (!effectivePrepared || busy || preflight.current || disabled || storageBlocked) return;
    let confirmed = effectivePrepared;
    if (!storedIntent) {
      const controller = new AbortController();
      preflight.current = controller;
      setChecking(true);
      setValidation(null);
      try {
        confirmed = await refreshTransferReview(effectivePrepared, controller.signal);
        if (controller.signal.aborted) return;
        if (reviewAccountChanged(effectivePrepared.source, confirmed.source) || reviewAccountChanged(effectivePrepared.destination, confirmed.destination)) {
          setPrepared(confirmed);
          setValidation("Account details changed. Review the updated balances and confirm again. No transfer was submitted.");
          return;
        }
      } catch (error) {
        if (!controller.signal.aborted) setValidation(error instanceof Error ? error.message : "We couldn’t recheck the accounts. No transfer was submitted.");
        return;
      } finally { preflight.current = null; setChecking(false); }
    }
    if (!prepared) {
      setPrepared(confirmed);
      setSource(confirmed.source.account_id);
      setDestination(confirmed.destination.account_id);
      setAmount(decimalFromMinorUnits(confirmed.source.currency, confirmed.amountMinor));
    }
    if (await postPrepared(confirmed)) { setPrepared(null); setAmount(""); }
  }

  if (outcome?.kind === "success") {
    return frame(<TransferResult outcome={outcome} headingRef={outcomeHeading} canStartAnother={!storageBlocked && !storedIntent} onAnother={() => setOutcome(null)} />, "result");
  }

  if (storedIntent && !restorableIntent) {
    return frame(<StatePanel
      kind="denied"
      title="Unconfirmed transfer cannot be restored"
      message="The original accounts are no longer both available in the authorized active scope. LedgerSync will not alter or recreate this intent with a different key. Inspect transfer history before taking further action."
    />, "result");
  }

  if (effectivePrepared) {
    const outcomeUnknown = outcome?.kind === "unknown";
    const submissionDisabled = Boolean(disabled || storageBlocked || (!outcomeUnknown && pickerUnavailable));
    const availability: ActionAvailabilityStatus = busy
      ? { state: "busy", reason: checking ? "Checking the latest account details." : "Wait while this transfer request finishes." }
      : storageBlocked ? { state: "temporary_unavailable", reason: "Browser retry information must be available before submitting." }
      : submissionDisabled ? preparationAvailability : { state: "available" };
    return frame(<section className="transfer-review" aria-labelledby="transfer-review-heading">
      <h2 ref={reviewHeading} tabIndex={-1} id="transfer-review-heading" className={outcome ? undefined : "sr-only"}>{outcomeUnknown ? "Result not yet confirmed" : "Review transfer"}</h2>
      {outcome && <StatePanel announce="assertive" kind={outcomeUnknown ? "unknown" : "error"} title={outcomeUnknown ? "Do not create another transfer" : "Transfer not completed"} message={outcome.message} />}
      <TransferReview transfer={effectivePrepared} unresolved={outcomeUnknown} />
      {validation && <p className="field-error" role="alert">{validation}</p>}
      {outcome?.requestReference && <TechnicalDetails summary="View request details"><RecordIdentity label="Request reference" value={outcome.requestReference} /></TechnicalDetails>}
      {accountsError && <StatePanel kind="error" title="Account check unavailable" message="We couldn’t refresh the accounts. Previously checked information may be out of date." action={<FocusedRetry label="Retry account check" onRetry={onRetryAccounts} disabled={disabled} busy={accountsLoading} />} />}
      <div className="command-actions">
        {outcomeUnknown ? <p className="intent-lock-note">Editing is locked until this original request is resolved.</p> : <ActionAvailability availability={busy ? { state: "busy", reason: "Wait for the current account check or request." } : { state: "available" }}><button className="button secondary" type="button" onClick={() => { setPrepared(null); setOutcome(null); setValidation(null); }}>Back to edit</button></ActionAvailability>}
        <ActionAvailability availability={availability}><button className="button primary" type="button" onClick={() => void submit()}>{checking ? "Checking accounts…" : pending ? "Submitting transfer…" : outcomeUnknown ? "Retry this same request safely" : "Confirm transfer"}</button></ActionAvailability>
      </div>
    </section>, outcome ? "result" : "review");
  }

  if (accountsLoading && accounts.length === 0) {
    return frame(<StatePanel title="Checking your accounts" message="Wait while we check which accounts are available for this transfer." />, "details");
  }

  if (accountsError && accounts.length === 0) {
    return frame(<StatePanel kind="error" title="Accounts could not be checked" message="No transfer has been submitted. Try checking the accounts again." action={<FocusedRetry label="Retry account check" onRetry={onRetryAccounts} disabled={disabled} busy={accountsLoading} />} />, "details");
  }

  if (preferredFundingBlocked || transferable.length < 2 || fundedSources.length === 0 || destinations.length === 0) {
    return frame(<StatePanel kind="denied" title="You need two eligible accounts" message="Choose different active accounts in the same currency, with enough available money in the source account." action={<Link className="button secondary" href="/accounts">View accounts</Link>} />, "details");
  }

  return frame(<section className="transfer-form" aria-labelledby="transfer-heading">
    <div>

      <h2 id="transfer-heading">Transfer details</h2>
      <p className="muted">Choose where the money will move. You’ll review before confirming.</p>
    </div>
    {accountsVerifiedAt && <EvidenceFreshness state={accountsError ? "historical" : accountsLoading ? "refreshing" : "current"} verifiedAt={accountsVerifiedAt} label="Account picker" reason={accountsError ?? undefined} />}
    {accountsError && <StatePanel kind="error" title="Account picker not refreshed" message={accountsError} action={<FocusedRetry label="Retry account picker only" onRetry={onRetryAccounts} disabled={disabled} busy={accountsLoading} />} />}
    <form onSubmit={prepare} noValidate>
      <FormField label="From account" requirement="required" hint="Money will leave this account."><select value={effectiveSource} onChange={(event) => { userChangedRoute.current = true; setSource(event.target.value); }} disabled={disabled || pickerUnavailable} required>{fundedSources.map((account) => <option key={account.account_id} value={account.account_id}>{accountLabel(account)} · {account.currency}</option>)}</select></FormField>
      <FormField label="To account" requirement="required" hint="Money will go to this account."><select value={effectiveDestination} onChange={(event) => { userChangedRoute.current = true; setDestination(event.target.value); }} disabled={disabled || pickerUnavailable} required>{destinations.map((account) => <option key={account.account_id} value={account.account_id}>{accountLabel(account)} · {account.currency}</option>)}</select></FormField>
      <FormField label="Amount" requirement="required" hint={`Enter ${sourceAccount?.currency ?? "the account currency"}, for example 1250.00.`}><input value={amount} onChange={(event) => setAmount(event.target.value)} inputMode="decimal" autoComplete="off" placeholder="1250.00" aria-describedby="transfer-error transfer-disabled-reason" aria-invalid={Boolean(validation)} disabled={disabled || pickerUnavailable} required /></FormField>
      {preferredDestination && !storedIntent && <p className="destination-preselection" role="status">Destination preselected from account <code>{preferredDestination.account_id}</code>. Review the source and exact amount before posting.</p>}
      {validation && <p id="transfer-error" className="field-error" role="alert">{validation}</p>}
      <ActionAvailability availability={storageBlocked ? { state: "temporary_unavailable", reason: "Allow browser storage before preparing a request." } : preparationAvailability}><button className="button primary" type="submit">Review transfer</button></ActionAvailability>
      {(disabled || pickerUnavailable) && <p id="transfer-disabled-reason" className="permission-note">{prerequisiteReason ?? "Transfer preparation is disabled until connectivity and authorization are verified."}</p>}
    </form>
  </section>, "details");
}
