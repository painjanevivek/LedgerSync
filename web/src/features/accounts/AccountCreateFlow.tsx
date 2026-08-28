"use client";

import { CheckCircle, LockKey, WarningCircle } from "@phosphor-icons/react";
import Link from "next/link";
import { type FormEvent, useEffect, useMemo, useRef, useState } from "react";

import {
  accountCategories,
  createAccountStorageKey,
  newAccountIdempotencyKey,
  normalizeCreateAccountFields,
  parseCreateAccountIntent,
  validAccountDisplayName,
  validAccountExternalReference,
  type CreateAccountFields,
  type CreateAccountIntent,
} from "@/features/accounts/accountCommandIntent";
import { useAccountCommand } from "@/features/accounts/useAccountCommand";
import { CopyControl, PageHeader, StatePanel, StatusBadge } from "@/features/console/components";
import { formatMinorUnits } from "@/lib/money";

type Props = Readonly<{
  tenantId: string;
  tenantLabel: string;
  environmentLabel: string;
  csrfToken: string;
  online: boolean;
  canWrite: boolean;
  canTransfer: boolean;
  fundingScopeComplete: boolean;
  fundedSourceAvailable: boolean;
  returnTo: string;
  onCreated: () => Promise<void>;
}>;

const categoryLabels: Record<(typeof accountCategories)[number], string> = {
  operating: "Operating",
  customer_funds: "Customer funds",
  payroll: "Payroll",
  payables: "Payables",
  expenses: "Expenses",
  reserve: "Reserve",
};

function newIntent(tenantId: string): CreateAccountIntent {
  return {
    version: 1,
    kind: "create",
    tenantId,
    idempotencyKey: newAccountIdempotencyKey(),
    stage: "identity",
    request: { display_name: "", external_reference: "", category: "operating", currency: "INR" },
  };
}

function validateIdentity(fields: CreateAccountFields) {
  const errors: string[] = [];
  if (!validAccountDisplayName(fields.display_name)) errors.push("Display name must be 1–120 characters without control characters.");
  if (!validAccountExternalReference(fields.external_reference)) errors.push("External reference must be 3–64 letters, numbers, dots, underscores, or hyphens, beginning with a letter or number.");
  return errors;
}

export function AccountCreateFlow({ tenantId, tenantLabel, environmentLabel, csrfToken, online, canWrite, canTransfer, fundingScopeComplete, fundedSourceAvailable, returnTo, onCreated }: Props) {
  const storageKey = useMemo(() => createAccountStorageKey(tenantId), [tenantId]);
  const [intent, setIntent] = useState<CreateAccountIntent>(() => newIntent(tenantId));
  const [restored, setRestored] = useState(false);
  const [errors, setErrors] = useState<string[]>([]);
  const { pending, outcome, setOutcome, send } = useAccountCommand(csrfToken);
  const stageHeading = useRef<HTMLHeadingElement>(null);
  const errorHeading = useRef<HTMLDivElement>(null);
  const abandonDialog = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      const stored = parseCreateAccountIntent(sessionStorage.getItem(storageKey), tenantId);
      if (stored) setIntent(stored);
      setRestored(true);
    });
    return () => cancelAnimationFrame(frame);
  }, [storageKey, tenantId]);

  useEffect(() => {
    if (!restored || outcome?.kind === "success") return;
    sessionStorage.setItem(storageKey, JSON.stringify(intent));
  }, [intent, outcome, restored, storageKey]);

  useEffect(() => { stageHeading.current?.focus(); }, [intent.stage, outcome?.kind]);

  function updateRequest(field: keyof CreateAccountFields, value: string) {
    setErrors([]);
    setOutcome(null);
    setIntent((current) => ({ ...current, request: { ...current.request, [field]: value } as CreateAccountFields }));
  }

  function submitIdentity(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalized = normalizeCreateAccountFields(intent.request);
    const nextErrors = validateIdentity(normalized);
    setErrors(nextErrors);
    if (nextErrors.length) { errorHeading.current?.focus(); return; }
    setIntent((current) => ({ ...current, request: normalized, stage: "boundary" }));
  }

  async function createAccount() {
    if (pending || !online || !canWrite) return;
    const locked = { ...intent, stage: "unknown" as const };
    sessionStorage.setItem(storageKey, JSON.stringify(locked));
    setIntent(locked);
    const result = await send("/api/me/accounts", "POST", locked.request, locked.idempotencyKey);
    if (result.kind === "success") {
      sessionStorage.removeItem(storageKey);
      await onCreated();
    } else if (result.kind !== "unknown") {
      setIntent((current) => ({ ...current, stage: "review", idempotencyKey: newAccountIdempotencyKey() }));
    }
  }

  function abandon() {
    sessionStorage.removeItem(storageKey);
    setOutcome(null);
    setErrors([]);
    setIntent(newIntent(tenantId));
    abandonDialog.current?.close();
  }

  const success = outcome?.kind === "success" ? outcome : null;
  const failure = outcome && outcome.kind !== "success" ? outcome : null;
  const stepIndex = intent.stage === "identity" ? 0 : intent.stage === "boundary" ? 1 : 2;

  return <>
    <PageHeader eyebrow="Ledger / Account command" title="Create account" description="Define one ledger account identity, verify its zero-INR financial boundary, then submit one retry-safe command.">
      <Link className="button secondary" href={returnTo}>Return to accounts</Link>
    </PageHeader>
    {!canWrite && <StatePanel kind="denied" title="Account creation unavailable" message="Your current role does not include account command authority." />}
    <nav className="command-steps" aria-label="Account creation progress">
      {["Identity", "Financial boundary", "Review", "Result"].map((label, index) => <span key={label} className={index <= stepIndex || success && index === 3 ? "current" : ""} aria-current={index === stepIndex && !success ? "step" : undefined}><b>{index + 1}</b>{label}</span>)}
    </nav>
    {errors.length > 0 && <div ref={errorHeading} tabIndex={-1} className="error-summary" role="alert"><strong>Check the account identity</strong><ul>{errors.map((error) => <li key={error}>{error}</li>)}</ul></div>}
    {success ? <section className="surface account-command-result" aria-labelledby="account-result-heading" role="region">
      <p className="eyebrow">Verified command result</p>
      <h2 ref={stageHeading} tabIndex={-1} id="account-result-heading">Account created</h2>
      <StatePanel title={success.replayed ? "Existing result safely replayed" : "Account identity committed"} message="The account begins at exactly INR 0.00. No money moved during creation." />
      <dl className="evidence-list">
        <div><dt>Account ID</dt><dd><CopyControl value={success.account.account_id} /></dd></div>
        <div><dt>External reference</dt><dd>{success.account.external_reference}</dd></div>
        <div><dt>Status</dt><dd><StatusBadge tone="success">{success.account.status}</StatusBadge></dd></div>
        <div><dt>Account version</dt><dd><code>{success.account.account_version}</code></dd></div>
        <div><dt>Exact balance</dt><dd>{formatMinorUnits("INR", success.account.available_minor)}</dd></div>
        {success.requestReference && <div><dt>Request reference</dt><dd><CopyControl value={success.requestReference} label="Copy request reference" /></dd></div>}
      </dl>
      <div className="action-row account-command-actions">
        {success.account.status === "active" && canTransfer && fundingScopeComplete && fundedSourceAvailable && <Link className="button primary" href={`/transfers?destination=${encodeURIComponent(success.account.account_id)}&return_to=${encodeURIComponent(`/accounts/${success.account.account_id}`)}`}>Fund account</Link>}
        <Link className="button secondary" href={`/accounts/${encodeURIComponent(success.account.account_id)}?return_to=${encodeURIComponent(returnTo)}`}>View account</Link>
      </div>
      {!canTransfer && <p className="permission-note">Your current role can configure accounts but cannot post the separate transfer required to fund one.</p>}
      {canTransfer && !fundingScopeComplete && <p className="permission-note">The authorized account picker exceeds its bounded scope. LedgerSync cannot prove a funded source, so funding remains unavailable.</p>}
      {canTransfer && fundingScopeComplete && !fundedSourceAvailable && <p className="permission-note">No different active, authorized INR source has a positive available balance. Funding remains unavailable.</p>}
    </section> : intent.stage === "identity" ? <section className="surface account-create-document" aria-labelledby="identity-heading">
      <p className="eyebrow">Step 1 of 4 · Identity</p>
      <h2 ref={stageHeading} tabIndex={-1} id="identity-heading">Define the account record</h2>
      <p className="muted">These fields identify the ledger boundary. They do not create or edit a balance.</p>
      <form className="account-command-form" onSubmit={submitIdentity} noValidate>
        <label>Display name<input value={intent.request.display_name} onChange={(event) => updateRequest("display_name", event.target.value)} maxLength={120} autoComplete="off" required /></label>
        <label>External reference<input value={intent.request.external_reference} onChange={(event) => updateRequest("external_reference", event.target.value)} maxLength={64} pattern="[A-Za-z0-9][A-Za-z0-9._-]{2,63}" autoComplete="off" required /></label>
        <label>Category<select value={intent.request.category} onChange={(event) => updateRequest("category", event.target.value)}>{accountCategories.map((category) => <option key={category} value={category}>{categoryLabels[category]}</option>)}</select></label>
        <label>Currency<input value="INR" readOnly aria-describedby="currency-boundary-help" /></label>
        <p id="currency-boundary-help" className="muted">This local ledger supports the fixed INR boundary for this flow.</p>
        <button className="button primary guarded-control" type="submit" disabled={!canWrite}>Continue to financial boundary</button>
      </form>
    </section> : intent.stage === "boundary" ? <section className="surface account-create-document" aria-labelledby="boundary-heading">
      <p className="eyebrow">Step 2 of 4 · Financial boundary</p>
      <h2 ref={stageHeading} tabIndex={-1} id="boundary-heading">Verify what creation cannot do</h2>
      <div className="financial-boundary-proof" aria-label="Account financial boundary proof">
        <div><LockKey weight="fill" aria-hidden="true" /><span>Currency boundary</span><strong>INR · fixed</strong></div>
        <div><CheckCircle weight="fill" aria-hidden="true" /><span>Opening balance</span><strong>INR 0.00 · exact</strong></div>
        <p><span>Record created</span><strong>PostgreSQL-backed account identity</strong></p>
        <p><span>Money movement</span><strong>None — funding requires a separate ledger transfer</strong></p>
      </div>
      <p className="guardrail-copy">Creating an account does not create a balance. Opening value can only arrive through a separately authorized, auditable ledger transaction.</p>
      <div className="action-row account-command-actions"><button className="button secondary guarded-control" type="button" onClick={() => setIntent((current) => ({ ...current, stage: "identity" }))}>Back to identity</button><button className="button primary guarded-control" type="button" onClick={() => setIntent((current) => ({ ...current, stage: "review" }))}>Continue to review</button></div>
    </section> : <section className="surface account-create-document" aria-labelledby="review-heading">
      <p className="eyebrow">Step 3 of 4 · Review</p>
      <h2 ref={stageHeading} tabIndex={-1} id="review-heading">Review exact account command</h2>
      <dl className="review-grid account-review-grid">
        <div><dt>Display name</dt><dd>{intent.request.display_name}</dd></div>
        <div><dt>External reference</dt><dd><code>{intent.request.external_reference}</code></dd></div>
        <div><dt>Category</dt><dd>{categoryLabels[intent.request.category]}</dd></div>
        <div><dt>Currency</dt><dd>INR</dd></div>
        <div><dt>Tenant</dt><dd>{tenantLabel}<code>{tenantId}</code></dd></div>
        <div><dt>Environment</dt><dd>{environmentLabel}</dd></div>
      </dl>
      <div className="financial-boundary-proof compact"><p><span>Account begins</span><strong>INR 0.00</strong></p><p><span>This command moves</span><strong>No money</strong></p></div>
      {failure && <StatePanel kind={failure.kind === "unknown" ? "unknown" : failure.kind === "denied" ? "denied" : "error"} title={failure.kind === "unknown" ? "Account result not yet confirmed" : "Account was not created"} message={failure.message} />}
      {!online && <StatePanel kind="offline" title="Offline — command retained" message="The reviewed account identity is stored for this tenant on this device. Reconnect to submit it." />}
      <div className="action-row account-command-actions">
        {intent.stage === "unknown" || failure?.kind === "unknown" ? <><p className="intent-lock-note"><WarningCircle weight="fill" aria-hidden="true" /> Editing is locked while this exact outcome is unresolved.</p><button className="button secondary guarded-control" type="button" onClick={() => abandonDialog.current?.showModal()}>Abandon local recovery</button></> : <button className="button secondary guarded-control" type="button" disabled={pending} onClick={() => setIntent((current) => ({ ...current, stage: "identity" }))}>Back to edit</button>}
        <button className="button primary guarded-control" type="button" disabled={pending || !online || !canWrite} onClick={() => void createAccount()}>{pending ? "Submitting account…" : intent.stage === "unknown" || failure?.kind === "unknown" ? "Retry same account command" : "Create account"}</button>
      </div>
    </section>}
    <dialog ref={abandonDialog} className="confirmation-dialog" aria-labelledby="abandon-heading" onCancel={() => abandonDialog.current?.close()}>
      <form method="dialog"><p className="eyebrow">Recovery decision</p><h2 id="abandon-heading">Abandon local account recovery?</h2><p>This removes the local retry record. It does not cancel a command that may already have committed. Inspect the account directory before creating another account with this external reference.</p><div className="action-row account-command-actions"><button className="button secondary guarded-control" value="cancel">Keep recovery</button><button className="button danger guarded-control" value="confirm" onClick={abandon}>Abandon local recovery</button></div></form>
    </dialog>
  </>;
}
