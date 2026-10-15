"use client";

import { WarningCircle } from "@phosphor-icons/react";
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
import { CopyControl } from "@/ui/controls/CopyControl.client";
import { CommandFrame } from "@/ui/presentation/CommandFrame";
import { TechnicalDetails } from "@/ui/presentation/TechnicalDetails";
import { ActionAvailability } from "@/ui/controls/ActionAvailability";
import { StatePanel } from "@/ui/display/StatePanel";
import { StatusBadge } from "@/ui/display/StatusBadge";
import { FormField } from "@/ui/forms/FormField.client";
import { Money } from "@/ui/display/Money";

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
  const [storageBlocked, setStorageBlocked] = useState(false);

  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      try {
        const raw = sessionStorage.getItem(storageKey);
        const stored = parseCreateAccountIntent(raw, tenantId);
        if (raw && !stored) { setStorageBlocked(true); }
        else if (stored) setIntent({ ...stored, stage: stored.stage === "boundary" ? "review" : stored.stage });
      } catch { setStorageBlocked(true); }
      setRestored(true);
    });
    return () => cancelAnimationFrame(frame);
  }, [storageKey, tenantId]);

  useEffect(() => {
    if (!restored || storageBlocked || outcome?.kind === "success") return;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- Failed external persistence must immediately block financial submission.
    try { sessionStorage.setItem(storageKey, JSON.stringify(intent)); } catch { setStorageBlocked(true); }
  }, [intent, outcome, restored, storageKey, storageBlocked]);

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
    setIntent((current) => ({ ...current, request: normalized, stage: "review" }));
  }

  async function createAccount() {
    if (pending || !online || !canWrite || !restored || storageBlocked) return;
    const locked = { ...intent, stage: "unknown" as const };
    try {
      sessionStorage.setItem(storageKey, JSON.stringify(locked));
      if (sessionStorage.getItem(storageKey) !== JSON.stringify(locked)) throw new Error("Storage unavailable");
    } catch { setStorageBlocked(true); return; }
    setIntent(locked);
    const result = await send("/api/me/accounts", "POST", locked.request, locked.idempotencyKey);
    if (result.kind === "success") {
      try { sessionStorage.removeItem(storageKey); } catch { setStorageBlocked(true); }
      await onCreated().catch(() => undefined);
    } else if (result.kind !== "unknown") {
      setIntent((current) => ({ ...current, stage: "review", idempotencyKey: newAccountIdempotencyKey() }));
    }
  }

  const success = outcome?.kind === "success" ? outcome : null;
  const failure = outcome && outcome.kind !== "success" ? outcome : null;
  const stage = success || intent.stage === "unknown" || failure ? "result" : intent.stage === "identity" ? "details" : "review";

  return <CommandFrame title="Create an account." description="Give the account a clear purpose, review its details, then confirm." stage={stage} returnTo={returnTo} returnLabel="Back to accounts" help={<><p>Every new account starts at exactly INR 0.00.</p><p>Creating an account does not move money or make a bank deposit. Add money later through a separate reviewed record.</p></>}>
    {!canWrite && <StatePanel kind="denied" title="Account creation unavailable" message="Your current role does not include account command authority." />}
    {storageBlocked && <StatePanel kind="error" title="Account retry information is unavailable" message="No new command can be submitted. Allow browser storage, then reload. Retained request information has not been replaced." />}
    {errors.length > 0 && <div ref={errorHeading} tabIndex={-1} className="error-summary" role="alert"><strong>Check the account identity</strong><ul>{errors.map((error) => <li key={error}>{error}</li>)}</ul></div>}
    {success ? <section className="surface account-command-result" aria-labelledby="account-result-heading" role="region">
      <p className="eyebrow">Verified command result</p>
      <h2 ref={stageHeading} tabIndex={-1} id="account-result-heading">Account created</h2>
      <StatePanel title={success.replayed ? "Existing result safely replayed" : "Account identity committed"} message="The account begins at exactly INR 0.00. No money moved during creation." />
      <p><strong>{success.account.display_name}</strong> starts at <Money currency="INR" minorUnits={success.account.available_minor} />.</p>
      <TechnicalDetails summary="View account details"><dl className="evidence-list">
        <div><dt>Account ID</dt><dd><CopyControl value={success.account.account_id} /></dd></div>
        <div><dt>External reference</dt><dd>{success.account.external_reference}</dd></div>
        <div><dt>Status</dt><dd><StatusBadge tone="success">{success.account.status}</StatusBadge></dd></div>
        <div><dt>Account version</dt><dd><code>{success.account.account_version}</code></dd></div>
        <div><dt>Exact balance</dt><dd><Money currency="INR" minorUnits={success.account.available_minor} /></dd></div>
        {success.requestReference && <div><dt>Request reference</dt><dd><CopyControl value={success.requestReference} label="Copy request reference" /></dd></div>}
      </dl></TechnicalDetails>
      <div className="action-row account-command-actions">
        {success.account.status === "active" && canTransfer && fundingScopeComplete && fundedSourceAvailable && <Link className="button primary" href={`/transfers?destination=${encodeURIComponent(success.account.account_id)}&return_to=${encodeURIComponent(`/accounts/${success.account.account_id}`)}`}>Fund account</Link>}
        <Link className="button secondary" href={`/accounts/${encodeURIComponent(success.account.account_id)}?return_to=${encodeURIComponent(returnTo)}`}>View account</Link>
      </div>
      {!canTransfer && <p className="permission-note">Your current role can configure accounts but cannot post the separate transfer required to fund one.</p>}
      {canTransfer && !fundingScopeComplete && <p className="permission-note">The authorized account picker exceeds its bounded scope. LedgerSync cannot prove a funded source, so funding remains unavailable.</p>}
      {canTransfer && fundingScopeComplete && !fundedSourceAvailable && <p className="permission-note">No different active, authorized INR source has a positive available balance. Funding remains unavailable.</p>}
    </section> : intent.stage === "identity" ? <section className="surface account-create-document" aria-labelledby="identity-heading">
      <p className="eyebrow">Details</p>
      <h2 ref={stageHeading} tabIndex={-1} id="identity-heading">Account details</h2>
      <p className="muted">Choose a recognizable name and purpose. The account will start with no money.</p>
      <form className="account-command-form" onSubmit={submitIdentity} noValidate>
        <FormField label="Display name" requirement="required" hint="Use a name people will recognize in LedgerSync."><input value={intent.request.display_name} onChange={(event) => updateRequest("display_name", event.target.value)} maxLength={120} autoComplete="off" required /></FormField>
        <FormField label="External reference" requirement="required" hint="Use your own stable reference. Example: ACME-OPERATING-01."><input value={intent.request.external_reference} onChange={(event) => updateRequest("external_reference", event.target.value)} maxLength={64} pattern="[A-Za-z0-9][A-Za-z0-9._-]{2,63}" autoComplete="off" required /></FormField>
        <FormField label="Category" requirement="required" hint="Choose the account’s main purpose."><select value={intent.request.category} onChange={(event) => updateRequest("category", event.target.value)} required>{accountCategories.map((category) => <option key={category} value={category}>{categoryLabels[category]}</option>)}</select></FormField>
        <label>Currency<input value="INR" readOnly aria-describedby="currency-boundary-help" /></label>
        <p id="currency-boundary-help" className="muted">This account uses INR. Its opening balance will be INR 0.00.</p>
        <ActionAvailability availability={!canWrite ? { state: "capability_missing", reason: "Your role does not allow account creation." } : storageBlocked || !restored ? { state: "temporary_unavailable", reason: "Retry information must be available first." } : { state: "available" }}><button className="button primary guarded-control" type="submit">Review account</button></ActionAvailability>
      </form>
    </section> : <section className="surface account-create-document" aria-labelledby="review-heading">
      <p className="eyebrow">Review</p>
      <h2 ref={stageHeading} tabIndex={-1} id="review-heading">Check your account details</h2>
      <dl className="review-grid account-review-grid">
        <div><dt>Display name</dt><dd>{intent.request.display_name}</dd></div>
        <div><dt>External reference</dt><dd><code>{intent.request.external_reference}</code></dd></div>
        <div><dt>Category</dt><dd>{categoryLabels[intent.request.category]}</dd></div>
        <div><dt>Currency</dt><dd>INR</dd></div>
        <div><dt>Workspace</dt><dd>{tenantLabel}</dd></div>
        <div><dt>Environment</dt><dd>{environmentLabel}</dd></div>
      </dl>
      <div className="financial-boundary-proof compact"><p><span>Account begins</span><strong>INR 0.00</strong></p><p><span>This command moves</span><strong>No money</strong></p></div>
      {failure && <StatePanel kind={failure.kind === "unknown" ? "unknown" : failure.kind === "denied" ? "denied" : "error"} title={failure.kind === "unknown" ? "Account result not yet confirmed" : "Account was not created"} message={failure.message} />}
      {!online && <StatePanel kind="offline" title="Offline — command retained" message="The reviewed account identity is stored for this tenant on this device. Reconnect to submit it." />}
      <div className="action-row account-command-actions">
        {intent.stage === "unknown" || failure?.kind === "unknown" ? <><p className="intent-lock-note"><WarningCircle weight="fill" aria-hidden="true" /> Editing is locked while this exact outcome is unresolved.</p><p>Returning to Accounts will not cancel or remove this original request. Resolve it before creating another.</p></> : <button className="button secondary guarded-control" type="button" disabled={pending} onClick={() => setIntent((current) => ({ ...current, stage: "identity" }))}>Back to edit</button>}
        <button className="button primary guarded-control" type="button" disabled={pending || !online || !canWrite || storageBlocked || !restored} onClick={() => void createAccount()}>{pending ? "Submitting account…" : intent.stage === "unknown" || failure?.kind === "unknown" ? "Retry this same account request safely" : "Create account"}</button>
      </div>
    </section>}
  </CommandFrame>;
}
