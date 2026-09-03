"use client";

import { CheckCircle, Compass, Info, LockKey, WarningCircle, X } from "@phosphor-icons/react";
import Link from "next/link";

import { CopyControl } from "@/ui/controls/CopyControl.client";
import { EvidenceFreshness } from "@/ui/display/Evidence";
import { StatusBadge } from "@/ui/display/StatusBadge";
import { Timestamp } from "@/ui/display/Timestamp";
import { canOpenOrientationStep, type ConsoleCapabilities } from "@/features/console/capabilities";
import type { LocalOrientation, OperatorPreferenceStepID, OrientationStep } from "@/lib/api/orientation";

type StepCopy = Readonly<{ title: string; description: string; href?: string; confirmation?: string }>;

const stepCopy: Record<OrientationStep["id"], StepCopy> = {
  confirm_health: { title: "Confirm local system health", description: "Check each dependency before treating financial evidence as current.", href: "/local-status", confirmation: "I checked current health" },
  understand_authority: { title: "Understand the authority boundary", description: "PostgreSQL owns ledger truth. Redis is disposable acceleration, never financial authority.", confirmation: "I understand the boundary" },
  inspect_accounts: { title: "Inspect your accounts", description: "Read an authorized account's exact INR balance and immutable history.", href: "/accounts", confirmation: "I inspected the account" },
  create_account: { title: "Create a zero-balance account", description: "Create an active INR account; value must enter through an approved ledger event.", href: "/accounts/new?return_to=%2Faccounts" },
  fund_account: { title: "Fund through an approved ledger event", description: "Record external value evidence, obtain the required finance decision, and post one balanced journal.", href: "/funding" },
  post_transfer: { title: "Transfer an exact amount", description: "Move integer minor units between eligible same-currency accounts.", href: "/transfers" },
  retry_transfer: { title: "Retry the same request safely", description: "Reuse the exact intent and idempotency key after an unknown response.", href: "/transfers", confirmation: "I verified the safe retry" },
  inspect_postings: { title: "Inspect postings and balance versions", description: "Follow the journal's equal debit and credit postings into versioned balance evidence.", href: "/transfers", confirmation: "I inspected the ledger proof" },
  run_reconciliation: { title: "Run reconciliation", description: "Compare ledger postings with PostgreSQL balance truth at a stored watermark.", href: "/reconciliation" },
  inspect_delivery: { title: "Inspect events and delivery", description: "Separate committed money from outbox publication and downstream delivery attempts.", href: "/events", confirmation: "I inspected delivery evidence" },
  export_evidence: { title: "Export bounded evidence", description: "Review the exact filter and row limit before downloading a sanitized CSV.", href: "/transfers", confirmation: "I reviewed an evidence export" },
  create_backup: { title: "Create and verify a backup", description: "Run the fixed host command, then confirm digest-bound recovery evidence.", href: "/recovery" },
};

const preferenceSteps = new Set<OrientationStep["id"]>(["confirm_health", "understand_authority", "inspect_accounts", "retry_transfer", "inspect_postings", "inspect_delivery", "export_evidence"]);

function evidenceHref(step: OrientationStep) {
  if (!step.evidence_id) return stepCopy[step.id].href;
  if (step.id === "inspect_accounts") return `/accounts/${encodeURIComponent(step.evidence_id)}`;
  if (step.id === "fund_account") return `/funding/${encodeURIComponent(step.evidence_id)}`;
  if (["post_transfer", "retry_transfer", "inspect_postings"].includes(step.id)) return `/transfers/${encodeURIComponent(step.evidence_id)}`;
  if (step.id === "run_reconciliation") return `/reconciliation/${encodeURIComponent(step.evidence_id)}`;
  if (step.id === "inspect_delivery") return `/events/${encodeURIComponent(step.evidence_id)}`;
  return stepCopy[step.id].href;
}

function complete(step: OrientationStep) { return step.state === "completed" || step.state === "operator_confirmed"; }
function tone(step: OrientationStep) { return complete(step) ? "success" as const : step.state === "evidence_available" ? "info" as const : "warning" as const; }
function stateLabel(step: OrientationStep) { return step.state === "completed" ? "Stored evidence" : step.state === "operator_confirmed" ? "Operator confirmed" : step.state === "evidence_available" ? "Ready to inspect" : step.state === "unavailable" ? "Unavailable" : "Not yet evidenced"; }
function reason(step: OrientationStep) {
  if (step.reason_code === "no_posted_funding_journal") return "No approved funding journal has been posted for this operator yet.";
  if (step.reason_code === "operator_confirmation_required") return "Open the evidence, then save an explicit operator confirmation.";
  if (step.reason_code === "no_authorized_account") return "No authorized account details are available yet.";
  if (step.reason_code === "no_posted_transfer") return "A posted transfer is required before this step can be verified.";
  if (step.reason_code === "no_delivery_attempt") return "No delivery attempt is available to inspect yet.";
  if (step.reason_code === "no_exportable_evidence") return "No authorized evidence is available to export yet.";
  return undefined;
}
function canConfirm(step: OrientationStep) {
  if (!preferenceSteps.has(step.id)) return false;
  return step.id === "confirm_health" || step.id === "understand_authority" || step.state === "evidence_available" || step.state === "operator_confirmed";
}

type PreferenceChange = Readonly<{ dismissed: boolean; completedStepIDs: OperatorPreferenceStepID[] }>;
type Props = Readonly<{
  evidence: LocalOrientation | null;
  loading: boolean;
  error: string | null;
  preferenceError: string | null;
  preferenceSaving: boolean;
  online: boolean;
  canRead: boolean;
  canWrite: boolean;
  capabilities: ConsoleCapabilities;
  forceOpen?: boolean;
  onRefresh: () => void;
  onUpdatePreferences: (change: PreferenceChange) => Promise<boolean>;
}>;

export function LocalOrientationPanel({ evidence, loading, error, preferenceError, preferenceSaving, online, canRead, canWrite, capabilities, forceOpen = false, onRefresh, onUpdatePreferences }: Props) {
  const completedStepIDs = evidence?.operator_completed_step_ids ?? [];
  const completedCount = evidence?.steps.filter(complete).length ?? 0;
  const hasIncompleteStep = evidence?.steps.some((step) => !complete(step)) ?? false;
  const nextStep = evidence?.steps.find((step) => !complete(step) && canOpenOrientationStep(capabilities, step.id));
  const writable = online && canWrite && !preferenceSaving;

  async function setDismissed(dismissed: boolean) {
    await onUpdatePreferences({ dismissed, completedStepIDs });
  }
  async function toggleStep(step: OrientationStep) {
    if (!preferenceSteps.has(step.id)) return;
    const stepID = step.id as OperatorPreferenceStepID;
    const next = step.state === "operator_confirmed" ? completedStepIDs.filter((id) => id !== stepID) : [...completedStepIDs, stepID];
    await onUpdatePreferences({ dismissed: false, completedStepIDs: next });
  }

  if (evidence?.dismissed && !forceOpen) return <section className="orientation-compact" aria-labelledby="orientation-compact-title">
    <Compass weight="fill" aria-hidden="true" />
    <div><p className="eyebrow">Setup progress · {completedCount}/12</p><h2 id="orientation-compact-title">{nextStep ? `Recommended next: ${stepCopy[nextStep.id].title}` : hasIncompleteStep ? "No authorized next action" : "Operator journey complete"}</h2><p>Progress is stored for this operator in PostgreSQL, not only in this browser.</p></div>
    <button className="button secondary" type="button" disabled={!writable} aria-describedby={!writable ? "orientation-preference-help" : undefined} onClick={() => void setDismissed(false)}>Reopen setup guide</button>
    {!writable && <p id="orientation-preference-help" className="permission-note">{!online ? "Reconnect to update setup preferences." : !canWrite ? "The local:write scope is required to update setup preferences." : "Saving setup preferences…"}</p>}
  </section>;

  return <section className="local-orientation" aria-labelledby="local-orientation-title">
    <header>
      <Compass weight="fill" aria-hidden="true" />
      <div><p className="eyebrow">Local operator journey / {completedCount} of 12 complete</p><h2 id="local-orientation-title">Follow one INR ledger record from system health to recovery</h2><p>Each green state names its source: authoritative stored evidence or an explicit operator confirmation. They are never interchangeable.</p></div>
      <button className="orientation-dismiss" type="button" disabled={!writable} aria-describedby={!writable ? "orientation-preference-help" : undefined} onClick={() => void setDismissed(true)} aria-label="Dismiss setup guide"><X aria-hidden="true" /></button>
    </header>
    <div className="orientation-boundary" aria-label="Local workspace boundaries">
      <div><strong>Currency</strong><span>INR only</span></div><div><strong>Movement</strong><span>Authorized internal accounts</span></div><div><strong>Authority</strong><span>PostgreSQL ledger</span></div><div><strong>Cache</strong><span>Redis is disposable</span></div>
    </div>
    {!canRead ? <div className="orientation-state"><WarningCircle weight="fill" aria-hidden="true"/><p><strong>Checklist permission required</strong><span>The guide remains readable, but durable progress needs the local:read scope.</span></p></div>
      : !evidence ? !online ? <div className="orientation-state"><WarningCircle weight="fill" aria-hidden="true"/><p><strong>Checklist unavailable offline</strong><span>No locally cached completion is presented as durable evidence.</span></p></div>
      : error ? <div className="orientation-state" role="alert"><WarningCircle weight="fill" aria-hidden="true"/><p><strong>Durable checklist unavailable</strong><span>{error}</span></p><button className="button secondary" type="button" onClick={onRefresh}>Retry evidence</button></div>
      : <><div className="orientation-state" aria-busy="true"><Info weight="fill" aria-hidden="true"/><p><strong>Loading durable progress</strong><span>LedgerSync is checking stored tenant evidence and operator preferences.</span></p></div><ol className="orientation-checklist orientation-checklist-loading" aria-label="Ledger journey loading">
        {(Object.keys(stepCopy) as OrientationStep["id"][]).map((stepId, index) => <li key={stepId}><span className="orientation-step-number" aria-hidden="true">{String(index + 1).padStart(2, "0")}</span><div><strong>{stepCopy[stepId].title}</strong><p>{stepCopy[stepId].description}</p><small>Stored evidence is being checked</small></div><StatusBadge tone="info">Checking evidence</StatusBadge></li>)}
      </ol></>
      : <><EvidenceFreshness state={error || !online ? "historical" : loading ? "refreshing" : "current"} verifiedAt={evidence.generated_at} label="Operator journey" reason={error ?? (!online ? "Reconnect before relying on checklist state." : undefined)} />
        <article className={`recommended-action ${nextStep?.state === "unavailable" || (!nextStep && hasIncompleteStep) ? "blocked" : ""}`} aria-labelledby="recommended-action-title"><div><p className="eyebrow">Recommended next action</p><h3 id="recommended-action-title">{nextStep ? stepCopy[nextStep.id].title : hasIncompleteStep ? "No authorized next action" : "Journey complete"}</h3><p>{nextStep ? (reason(nextStep) ?? stepCopy[nextStep.id].description) : hasIncompleteStep ? "Your current server-issued capabilities do not permit any remaining setup step. Ask an administrator for the required access; no protected request was made." : "Every available setup step has authoritative evidence or an explicit saved confirmation."}</p></div>{nextStep?.state === "unavailable" || (!nextStep && hasIncompleteStep) ? <span className="recommended-blocked"><LockKey weight="fill" aria-hidden="true" />Blocked safely</span> : nextStep && evidenceHref(nextStep) ? <Link className="button primary" href={evidenceHref(nextStep)!}>Open next step</Link> : nextStep && canConfirm(nextStep) ? <button className="button primary" type="button" disabled={!writable} onClick={() => void toggleStep(nextStep)}>{stepCopy[nextStep.id].confirmation}</button> : null}</article>
        <ol className="orientation-checklist">{evidence.steps.map((step, index) => <li key={step.id} className={nextStep?.id === step.id ? "current" : undefined}>
          <span className="orientation-step-number" aria-hidden="true">{String(index + 1).padStart(2, "0")}</span><span className={`orientation-check ${step.state}`} aria-hidden="true">{complete(step) ? <CheckCircle weight="fill" /> : step.state === "evidence_available" ? <Info weight="fill" /> : <WarningCircle weight="fill" />}</span>
          <div><strong>{stepCopy[step.id].title}</strong><p>{stepCopy[step.id].description}</p>{step.occurred_at ? <small>Stored evidence · <Timestamp value={step.occurred_at} /></small> : reason(step) ? <small>{reason(step)}</small> : step.state === "operator_confirmed" ? <small>Saved operator acknowledgement</small> : null}</div>
          <StatusBadge tone={tone(step)}>{stateLabel(step)}</StatusBadge><div className="orientation-step-actions">{canOpenOrientationStep(capabilities,step.id)&&evidenceHref(step) && step.state !== "unavailable" && <Link className="record-link" href={evidenceHref(step)!}>{step.evidence_id ? "Open evidence" : "Open step"}</Link>}{canOpenOrientationStep(capabilities,step.id)&&canConfirm(step) && <button className="record-link orientation-confirm" type="button" disabled={!writable} onClick={() => void toggleStep(step)}>{step.state === "operator_confirmed" ? "Undo confirmation" : stepCopy[step.id].confirmation}</button>}</div>
        </li>)}</ol>
      </>}
    {(preferenceError || !writable) && <div id="orientation-preference-help" className="orientation-preference-state" role={preferenceError ? "alert" : undefined}><Info weight="fill" aria-hidden="true"/><p>{preferenceError ?? (!online ? "Reconnect to update setup preferences." : !canWrite ? "The local:write scope is required to update setup preferences." : "Saving setup preferences…")}</p></div>}
    <details className="orientation-definitions"><summary>Plain-language ledger terms</summary><dl><div><dt>Idempotency</dt><dd>Retrying the same intent with the same key returns one durable outcome, not a second movement.</dd></div><div><dt>Double entry</dt><dd>Every posted journal has equal debit and credit postings in the same currency.</dd></div><div><dt>Projection</dt><dd>A derived balance view; PostgreSQL ledger records remain authoritative.</dd></div><div><dt>Reconciliation</dt><dd>A stored comparison between ledger postings and balance truth at a known watermark.</dd></div><div><dt>Response unknown</dt><dd>The response was lost, so the outcome must be checked or retried with the same key.</dd></div></dl></details>
    <footer><div><strong>Stop or restart without losing persisted progress</strong><p>Safe stop preserves PostgreSQL preferences. Restarting manual progress never deletes ledger, audit, or recovery evidence.</p></div><div className="orientation-footer-actions"><button className="button secondary" type="button" disabled={!writable || completedStepIDs.length === 0} onClick={() => void onUpdatePreferences({ dismissed: false, completedStepIDs: [] })}>Restart manual progress</button><CopyControl value="powershell -File .\scripts\stop-local.ps1" label="Copy safe stop command" /></div></footer>
  </section>;
}
