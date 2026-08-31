"use client";

import { useEffect, useMemo, useState } from "react";

import { CopyControl } from "@/ui/controls/CopyControl.client";
import { StatePanel } from "@/ui/display/StatePanel";
import { FormField } from "@/ui/forms/FormField.client";
import { newWebhookReplayIntent, parseWebhookReplayIntent, webhookReplayStorageKey, type WebhookReplayIntent } from "@/features/operations/webhookReplayIntent";
import { useWebhookReplay, type WebhookReplayOutcome } from "@/features/operations/useWebhookReplay";
import { isWebhookReplayIdentifier, webhookReplayReasonCodes, type WebhookReplayReasonCode } from "@/lib/api/webhook-replay";

function outcomePanel(outcome: WebhookReplayOutcome | null) {
  if (!outcome || outcome.kind === "approval" || outcome.kind === "scheduled") return null;
  return <StatePanel kind={outcome.kind === "unknown" ? "unknown" : outcome.kind === "denied" ? "denied" : "error"} title={outcome.kind === "unknown" ? "Replay command outcome unknown" : "Replay command not completed"} message={`${outcome.message} Request reference: ${outcome.requestReference}.`}/>;
}

export function WebhookReplayControl({ tenantId, endpointId, attemptId, csrfToken, online, canReplay, onScheduled }: Readonly<{ tenantId: string; endpointId: string; attemptId: string; csrfToken: string; online: boolean; canReplay: boolean; onScheduled: () => void }>) {
  const key = useMemo(() => webhookReplayStorageKey(tenantId, endpointId, attemptId), [attemptId, endpointId, tenantId]);
  const [intent, setIntent] = useState<WebhookReplayIntent | null>(null);
  const [reasonCode, setReasonCode] = useState<WebhookReplayReasonCode>("endpoint_restored");
  const [approvalId, setApprovalId] = useState("");
  const [outcome, setOutcome] = useState<WebhookReplayOutcome | null>(null);
  const { pending, send } = useWebhookReplay(csrfToken);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const restored = parseWebhookReplayIntent(sessionStorage.getItem(key), tenantId, endpointId, attemptId);
      if (restored) { setIntent(restored); setReasonCode(restored.reasonCode); setApprovalId(restored.approvalId ?? ""); }
    }, 0);
    return () => window.clearTimeout(timer);
  }, [attemptId, endpointId, key, tenantId]);

  function persist(next: WebhookReplayIntent) { sessionStorage.setItem(key, JSON.stringify(next)); setIntent(next); }
  function ensureIntent(): WebhookReplayIntent {
    if (intent) return intent;
    const next = newWebhookReplayIntent(tenantId, endpointId, attemptId, reasonCode); persist(next); return next;
  }

  async function approve() {
    const current = ensureIntent();
    const locked = current.state === "approval_unknown" ? current : { ...current, reasonCode, state: "review" as const };
    persist(locked); setOutcome(null);
    const result = await send(endpointId, attemptId, "approval", locked.approvalKey, { reasonCode: locked.reasonCode }); setOutcome(result);
    if (result.kind === "approval") { const next = { ...locked, approvalId: result.approvalId, state: "approved" as const }; persist(next); setApprovalId(result.approvalId); }
    else if (result.kind === "unknown") persist({ ...locked, state: "approval_unknown" });
  }

  async function execute() {
    if (!isWebhookReplayIdentifier(approvalId)) { setOutcome({ kind: "error", code: "invalid_approval", message: "Enter the exact approval ID supplied by the independent approving operator.", requestReference: "not-submitted" }); return; }
    const current = ensureIntent();
    const lockedApprovalId = current.state === "execution_unknown" ? current.approvalId : approvalId;
    if (!lockedApprovalId) return;
    const locked = { ...current, approvalId: lockedApprovalId, state: current.state === "execution_unknown" ? current.state : "approved" as const };
    persist(locked); setOutcome(null);
    const result = await send(endpointId, attemptId, "execution", locked.executionKey, { approvalId: lockedApprovalId }); setOutcome(result);
    if (result.kind === "scheduled") { persist({ ...locked, state: "scheduled", deliveryJobId: result.deliveryJobId }); onScheduled(); }
    else if (result.kind === "unknown") persist({ ...locked, state: "execution_unknown" });
  }

  const approvalLocked = intent?.state === "approval_unknown";
  const executionLocked = intent?.state === "execution_unknown";
  return <details className="ledger-section webhook-replay-control">
    <summary><strong>Controlled delivery replay</strong> — resend this existing event without changing financial state</summary>
    {!canReplay ? <StatePanel kind="denied" title="Replay permission not granted" message="This session can inspect delivery evidence but does not include webhooks:replay."/> : <>
      <div className="financial-separation-note"><div><strong>Two operators, one immutable event</strong><p>Approval and execution are separate, audited commands. Replay schedules another delivery of the existing event; it cannot edit the payload, destination, transfer, or ledger.</p></div></div>
      <section aria-labelledby={`approve-${attemptId}`}><h4 id={`approve-${attemptId}`}>1. Approving operator</h4><FormField label="Recovery reason" requirement="required" hint={approvalLocked ? "Locked because the previous approval outcome is unknown." : "Choose the verified operational condition that changed."}><select value={reasonCode} disabled={approvalLocked || pending !== null} onChange={(event) => setReasonCode(event.target.value as WebhookReplayReasonCode)}>{webhookReplayReasonCodes.map((reason) => <option key={reason} value={reason}>{reason.replaceAll("_", " ")}</option>)}</select></FormField><button className="button secondary guarded-control" type="button" disabled={!online || pending !== null} onClick={() => void approve()}>{pending === "approval" ? "Recording approval…" : approvalLocked ? "Retry exact approval" : "Record replay approval"}</button>{intent?.approvalId && <StatePanel title="Approval recorded for independent handoff" message="Copy this approval ID to the different operator who will execute the replay." action={<CopyControl value={intent.approvalId} label="Copy replay approval ID"/>}/>}</section>
      <section aria-labelledby={`execute-${attemptId}`}><h4 id={`execute-${attemptId}`}>2. Different executing operator</h4><FormField label="Approved command ID" requirement="required" hint={executionLocked ? "Locked because the previous execution outcome is unknown." : "Paste the exact approval ID handed off by the approving operator."}><input value={approvalId} readOnly={executionLocked} maxLength={36} autoComplete="off" onChange={(event) => setApprovalId(event.target.value.trim())}/></FormField><button className="button primary guarded-control" type="button" disabled={!online || pending !== null || !isWebhookReplayIdentifier(approvalId)} onClick={() => void execute()}>{pending === "execution" ? "Scheduling replay…" : executionLocked ? "Retry exact execution" : "Schedule existing event replay"}</button></section>
      {intent?.state === "scheduled" && intent.deliveryJobId && (
        <StatePanel title="Replay delivery scheduled" message="A new delivery job now references the immutable original event. Refresh endpoint evidence to follow its attempt; no financial posting was created." action={<CopyControl value={intent.deliveryJobId} label="Copy delivery job ID"/>}/>
      )}
      {outcomePanel(outcome)}
    </>}
  </details>;
}
