"use client";

import { ArrowClockwise, Snowflake, WarningCircle, XCircle } from "@phosphor-icons/react";
import Link from "next/link";
import { type FormEvent, useEffect, useMemo, useRef, useState } from "react";

import {
  lifecycleAccountStorageKey,
  newAccountIdempotencyKey,
  parseLifecycleAccountIntent,
  validLifecycleReason,
  type AccountTargetStatus,
  type LifecycleAccountIntent,
} from "@/features/accounts/accountCommandIntent";
import type { Account, AccountBalance } from "@/features/accounts/types";
import { useAccountCommand } from "@/features/accounts/useAccountCommand";
import { FormField, StatePanel } from "@/features/console/components";
import { formatMinorUnits } from "@/lib/money";

type Props = Readonly<{
  account: Account;
  balance: AccountBalance | null;
  balanceLoading: boolean;
  balanceError: string | null;
  tenantId: string;
  csrfToken: string;
  online: boolean;
  canWrite: boolean;
  canTransfer: boolean;
  fundingScopeComplete: boolean;
  fundedSourceAvailable: boolean;
  returnTo: string;
  onChanged: () => Promise<void>;
  onRefreshEvidence: () => Promise<{ account: Account | null; balance: AccountBalance | null }>;
}>;

function actionLabel(status: AccountTargetStatus) {
  return status === "frozen" ? "Freeze account" : status === "active" ? "Reactivate account" : "Close account";
}

function actionExplanation(status: AccountTargetStatus) {
  if (status === "frozen") return "New incoming and outgoing transfers will be rejected. Existing balance and immutable history remain unchanged.";
  if (status === "active") return "The account becomes eligible for authorized same-currency transfers again. This command does not move money.";
  return "Closure is terminal. The private API will recheck that the current exact balance is zero before committing.";
}

export function AccountLifecycleActions({ account, balance, balanceLoading, balanceError, tenantId, csrfToken, online, canWrite, canTransfer, fundingScopeComplete, fundedSourceAvailable, returnTo, onChanged, onRefreshEvidence }: Props) {
  const storageKey = useMemo(() => lifecycleAccountStorageKey(tenantId, account.account_id), [account.account_id, tenantId]);
  const [target, setTarget] = useState<AccountTargetStatus | null>(null);
  const [reason, setReason] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [validation, setValidation] = useState<string | null>(null);
  const [retained, setRetained] = useState<LifecycleAccountIntent | null>(null);
  const [commandEvidence, setCommandEvidence] = useState<{ account: Account | null; balance: AccountBalance | null } | null>(null);
  const [evidenceLoading, setEvidenceLoading] = useState(false);
  const [evidenceError, setEvidenceError] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const { pending, outcome, setOutcome, send } = useAccountCommand(csrfToken);
  const dialog = useRef<HTMLDialogElement>(null);
  const dialogTrigger = useRef<HTMLButtonElement | null>(null);
  const outcomeHeading = useRef<HTMLHeadingElement>(null);
  const dialogOutcome = useRef<HTMLDivElement>(null);
  const validationSummary = useRef<HTMLDivElement>(null);
  const refreshSequence = useRef(0);
  const exactZero = balance !== null && balance.available_minor === "0" && balance.ledger_minor === "0";
  const balanceCurrent = Boolean(balance) && !balanceLoading && !balanceError;
  const verifiedAccount = commandEvidence?.account?.account_id === account.account_id ? commandEvidence.account : null;
  const verifiedBalance = commandEvidence?.balance?.account_id === account.account_id && commandEvidence.balance.currency === account.currency ? commandEvidence.balance : null;
  const verifiedExactZero = verifiedBalance?.available_minor === "0" && verifiedBalance.ledger_minor === "0";

  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      const stored = parseLifecycleAccountIntent(sessionStorage.getItem(storageKey), tenantId, account.account_id);
      if (stored) setRetained(stored);
    });
    return () => cancelAnimationFrame(frame);
  }, [account.account_id, storageKey, tenantId]);

  useEffect(() => {
    if (!outcome) return;
    if (dialogOpen) dialogOutcome.current?.focus();
    else outcomeHeading.current?.focus();
  }, [dialogOpen, outcome]);

  async function open(nextTarget: AccountTargetStatus, trigger: HTMLButtonElement) {
    dialogTrigger.current = trigger;
    const sequence = ++refreshSequence.current;
    setTarget(nextTarget);
    setReason("");
    setConfirmation("");
    setValidation(null);
    setOutcome(null);
    setCommandEvidence(null);
    setEvidenceError(null);
    setEvidenceLoading(true);
    setDialogOpen(true);
    dialog.current?.showModal();
    const evidence = await onRefreshEvidence().catch(() => ({ account: null, balance: null }));
    if (sequence !== refreshSequence.current) return;
    setCommandEvidence(evidence);
    setEvidenceLoading(false);
    if (!evidence.account || evidence.account.account_id !== account.account_id) setEvidenceError("Current authoritative account configuration could not be verified. This command is disabled.");
    else if ((nextTarget === "frozen" && evidence.account.status !== "active") || (nextTarget === "active" && evidence.account.status !== "frozen") || (nextTarget === "closed" && evidence.account.status === "closed")) setEvidenceError("The authoritative account status changed. Cancel this dialog and review the refreshed account before issuing another command.");
    else if (nextTarget === "closed" && (!evidence.balance || evidence.balance.account_id !== account.account_id || evidence.balance.currency !== account.currency)) setEvidenceError("Current authoritative available and ledger balances could not be verified consistently. Closure is disabled.");
  }

  function showValidation(message: string) {
    setValidation(message);
    requestAnimationFrame(() => validationSummary.current?.focus());
  }

  async function submitIntent(intent: LifecycleAccountIntent) {
    sessionStorage.setItem(storageKey, JSON.stringify(intent));
    setRetained(intent);
    const result = await send(`/api/accounts/${encodeURIComponent(account.account_id)}`, "PATCH", intent.request, intent.idempotencyKey);
    if (result.kind === "success") {
      sessionStorage.removeItem(storageKey);
      setRetained(null);
      dialog.current?.close();
      await onChanged();
    } else if (result.kind !== "unknown") {
      sessionStorage.removeItem(storageKey);
      setRetained(null);
      if (result.kind === "conflict") {
        dialog.current?.close();
        await onChanged();
        requestAnimationFrame(() => outcomeHeading.current?.focus());
      }
    }
  }

  async function confirm(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!target || pending) return;
    if (retained && outcome?.kind === "unknown") { await submitIntent(retained); return; }
    if (!verifiedAccount || evidenceLoading || evidenceError) { showValidation("Refresh did not produce current, consistent account details. Cancel and try again when authoritative records are available."); return; }
    if (!validLifecycleReason(reason)) { showValidation("Reason is required and must be 1–256 characters without control characters."); return; }
    if (target === "closed" && confirmation !== verifiedAccount.external_reference) { showValidation("Enter the exact external reference to confirm terminal closure."); return; }
    if (target === "closed" && (!verifiedBalance || !verifiedExactZero)) { showValidation("Current authoritative available and ledger balances must both be exactly INR 0.00 before closure."); return; }
    const intent: LifecycleAccountIntent = {
      version: 1,
      kind: "lifecycle",
      tenantId,
      accountId: account.account_id,
      idempotencyKey: newAccountIdempotencyKey(),
      state: "unknown",
      request: { expected_version: verifiedAccount.account_version, target_status: target, reason },
    };
    await submitIntent(intent);
  }

  const recovery = retained && outcome?.kind !== "success";
  const actionDisabled = !online || !canWrite || pending || Boolean(recovery);
  const commandLocked = Boolean(retained && outcome?.kind === "unknown");

  return <section className="ledger-section account-lifecycle" aria-labelledby="account-controls-heading">
    <div className="section-heading"><div><p className="eyebrow">Configuration authority</p><h2 id="account-controls-heading">Account controls</h2><p>Lifecycle commands use configuration version <code>{account.account_version}</code>. Financial balance version <code>{balance?.version ?? "unavailable"}</code> is never used for configuration changes.</p></div></div>
    {account.status === "closed" ? <StatePanel title="Account lifecycle is terminal" message="Closed status cannot be reversed. Immutable transfer and audit history remains available below." /> : <>
      <div className="account-control-row">
        {account.status === "active" && <button className="button secondary guarded-control" type="button" disabled={actionDisabled} onClick={(event) => void open("frozen", event.currentTarget)}><Snowflake aria-hidden="true" />Freeze account</button>}
        {account.status === "frozen" && <button className="button secondary guarded-control" type="button" disabled={actionDisabled} onClick={(event) => void open("active", event.currentTarget)}><ArrowClockwise aria-hidden="true" />Reactivate account</button>}
        <button className="button danger guarded-control" type="button" disabled={actionDisabled} onClick={(event) => void open("closed", event.currentTarget)}><XCircle aria-hidden="true" />Close account</button>
        {account.status === "active" && canTransfer && fundingScopeComplete && fundedSourceAvailable && <Link className="button primary guarded-control" href={`/transfers?destination=${encodeURIComponent(account.account_id)}&return_to=${encodeURIComponent(returnTo)}`}>Fund account</Link>}
      </div>
      {!canWrite && <p className="permission-note">Read-only role: account lifecycle commands are not permitted.</p>}
      {account.status === "active" && !canTransfer && <p className="permission-note">Funding is unavailable because your role cannot post transfers.</p>}
      {account.status === "active" && canTransfer && !fundingScopeComplete && <p className="permission-note">The authorized account picker exceeds its bounded scope. LedgerSync cannot prove a funded source, so funding remains unavailable.</p>}
      {account.status === "active" && canTransfer && fundingScopeComplete && !fundedSourceAvailable && <p className="permission-note">No different active, authorized INR source has a positive available balance. Funding remains unavailable.</p>}
      {!online && <StatePanel kind="offline" title="Account controls are offline" message="Lifecycle commands are disabled until current account details can be verified and the command can be submitted." />}
      {balanceLoading && <StatePanel title="Verifying closure boundary" message="Current available and ledger balances are loading independently." />}
      {balanceError && <StatePanel kind="unknown" title="Closure details unavailable" message="Final closure confirmation remains disabled until the dialog refreshes and verifies both exact balances." />}
      {balanceCurrent && !exactZero && <StatePanel kind="denied" title="Close account requires exact zero" message={`Current spendable amount is ${account.currency} · ${balance!.available_minor} minor units; posted ledger amount is ${account.currency} · ${balance!.ledger_minor} minor units. Fund movement must use an auditable transfer; this control cannot edit either value.`} />}
    </>}
    {outcome && outcome.kind !== "success" && !recovery && !dialogOpen && <div className="account-command-recovery" role="region" aria-labelledby="lifecycle-result-heading"><h3 ref={outcomeHeading} tabIndex={-1} id="lifecycle-result-heading">Lifecycle command not completed</h3><StatePanel kind={outcome.kind === "denied" ? "denied" : "error"} title="Review current account details" message={outcome.message} /></div>}
    {recovery && !dialogOpen && <div className="account-command-recovery" role="region" aria-live="polite" aria-labelledby="lifecycle-recovery-heading">
      <h3 ref={outcomeHeading} tabIndex={-1} id="lifecycle-recovery-heading">Lifecycle result not yet confirmed</h3>
      <StatePanel kind="unknown" title="Exact command retained" message={outcome?.message ?? "A previous lifecycle submission may have committed. Editing is locked until this exact body and retry key are resolved."} />
      <dl className="review-grid"><div><dt>Command</dt><dd>{actionLabel(retained.request.target_status)}</dd></div><div><dt>Expected account version</dt><dd><code>{retained.request.expected_version}</code></dd></div><div><dt>Audited reason</dt><dd>{retained.request.reason}</dd></div></dl>
      <div className="action-row account-command-actions"><p className="intent-lock-note"><WarningCircle weight="fill" aria-hidden="true" /> Do not issue a different lifecycle command while this result is unknown.</p><button className="button primary guarded-control" type="button" disabled={pending || !online || !canWrite} onClick={() => void submitIntent(retained)}>{pending ? "Retrying command…" : `Retry same ${actionLabel(retained.request.target_status).toLowerCase()}`}</button></div>
    </div>}
    <dialog ref={dialog} className="confirmation-dialog" aria-labelledby="lifecycle-dialog-heading" aria-describedby="lifecycle-dialog-description" onClose={() => { refreshSequence.current += 1; setDialogOpen(false); setTarget(null); setValidation(null); setEvidenceLoading(false); requestAnimationFrame(() => dialogTrigger.current?.focus()); }}>
      {target && <form onSubmit={(event) => void confirm(event)} noValidate>
        <p className="eyebrow">Guarded lifecycle command</p>
        <h2 id="lifecycle-dialog-heading">{actionLabel(target)}</h2>
        <p id="lifecycle-dialog-description">{actionExplanation(target)} Current account configuration{target === "closed" ? " and both balance values are" : " is"} refreshed when this dialog opens.</p>
        {evidenceLoading && <StatePanel title="Refreshing account details" message="The command stays disabled until current account configuration and required balances are verified." />}
        {evidenceError && <StatePanel kind="unknown" title="Account details unavailable" message={evidenceError} />}
        <dl className="review-grid"><div><dt>Account</dt><dd>{verifiedAccount?.display_name || account.display_name || account.external_reference}<code>{account.account_id}</code></dd></div><div><dt>Current status</dt><dd>{verifiedAccount?.status ?? "Verifying"}</dd></div><div><dt>Expected account version</dt><dd><code>{verifiedAccount?.account_version ?? "Verifying"}</code></dd></div>{target === "closed" && <><div><dt>Available balance</dt><dd>{verifiedBalance ? formatMinorUnits(verifiedBalance.currency, verifiedBalance.available_minor) : "Unavailable"}</dd></div><div><dt>Ledger balance</dt><dd>{verifiedBalance ? formatMinorUnits(verifiedBalance.currency, verifiedBalance.ledger_minor) : "Unavailable"}</dd></div></>}</dl>
        <FormField label="Reason" requirement="required" hint="Explain why you are making this account change."><textarea value={reason} onChange={(event) => { setReason(event.target.value); setValidation(null); }} maxLength={256} rows={4} required disabled={commandLocked} aria-invalid={Boolean(validation)} aria-describedby={validation ? "lifecycle-validation" : undefined} /></FormField>
        {target === "closed" && <FormField label="Confirm external reference" requirement="required" hint={<>Enter <code>{account.external_reference}</code> exactly. Closing an account is final.</>}><input value={confirmation} onChange={(event) => { setConfirmation(event.target.value); setValidation(null); }} autoComplete="off" disabled={commandLocked} aria-invalid={Boolean(validation)} aria-describedby={validation ? "lifecycle-validation" : undefined} required /></FormField>}
        {validation && <div ref={validationSummary} tabIndex={-1} id="lifecycle-validation" className="error-summary" role="alert"><strong>Cannot submit lifecycle command</strong><p>{validation}</p></div>}
        {outcome && outcome.kind !== "success" && <div ref={dialogOutcome} tabIndex={-1} role="region" aria-label="Lifecycle command outcome"><StatePanel kind={outcome.kind === "unknown" ? "unknown" : outcome.kind === "denied" ? "denied" : "error"} title={outcome.kind === "unknown" ? "Result not confirmed" : "Lifecycle command rejected"} message={outcome.message} /></div>}
        <div className="action-row account-command-actions"><button className="button secondary guarded-control" type="button" disabled={pending} onClick={() => dialog.current?.close()}>Cancel</button><button className={target === "closed" ? "button danger guarded-control" : "button primary guarded-control"} type="submit" disabled={pending || !online || !canWrite || evidenceLoading || Boolean(evidenceError) || target === "closed" && (!verifiedBalance || !verifiedExactZero)}>{pending ? "Submitting command…" : commandLocked ? `Retry same ${actionLabel(target).toLowerCase()}` : `Confirm ${actionLabel(target).toLowerCase()}`}</button></div>
      </form>}
    </dialog>
  </section>;
}
