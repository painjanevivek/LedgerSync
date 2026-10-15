"use client";

import Link from "next/link";
import { useRef, useState } from "react";

import type { CorrectionStatus, TransferCorrection } from "@/lib/api/corrections";
import { useCorrectionCommand } from "@/features/corrections/useCorrectionCommand";
import { CopyControl } from "@/ui/controls/CopyControl.client";
import { DisclosureSection } from "@/ui/disclosure/DisclosureSection";
import { EvidenceFreshness } from "@/ui/display/Evidence";
import { Money } from "@/ui/display/Money";
import { PageHeader } from "@/ui/display/PageHeader";
import { StatePanel } from "@/ui/display/StatePanel";
import { StatusBadge } from "@/ui/display/StatusBadge";
import { Timestamp } from "@/ui/display/Timestamp";
import { FormField } from "@/ui/forms/FormField.client";
import { RelatedEvidenceRail } from "@/features/investigation/RelatedEvidenceRail";
import { ConfirmationDialog } from "@/ui/overlays/ConfirmationDialog.client";

function correctionTone(status: CorrectionStatus) {
  return status === "posted"
    ? ("success" as const)
    : status === "rejected" || status === "cancelled" || status === "expired"
      ? ("danger" as const)
      : ("warning" as const);
}

function label(value: string) {
  return value.replaceAll("_", " ");
}

export function CorrectionDetail({
  event,
  loading,
  error,
  verifiedAt,
  online,
  currentSubject,
  tenantId,
  csrfToken,
  canWrite,
  canApprove,
  actionBusy,
  decisionReason,
  stepUpRequired,
  returnTo,
  backHref,
  onReason,
  onRefresh,
  onAction,
}: Readonly<{
  event: TransferCorrection | null;
  loading: boolean;
  error: string | null;
  verifiedAt?: string;
  online: boolean;
  currentSubject: string;
  tenantId: string;
  csrfToken: string;
  canWrite: boolean;
  canApprove: boolean;
  actionBusy: boolean;
  decisionReason: string;
  stepUpRequired: boolean;
  returnTo: string;
  backHref: string;
  onReason: (value: string) => void;
  onRefresh: () => Promise<TransferCorrection | null>;
  onAction: (action: "approve" | "reject" | "cancel") => void;
}>) {
  const [postReviewOpen, setPostReviewOpen] = useState(false);
  const [postEvidence, setPostEvidence] = useState<TransferCorrection | null>(null);
  const [postVerificationError, setPostVerificationError] = useState<string | null>(null);
  const [verifyingPost, setVerifyingPost] = useState(false);
  const [restorePostTrigger, setRestorePostTrigger] = useState(true);
  const postTrigger = useRef<HTMLButtonElement>(null);
  const postOutcome = useRef<HTMLDivElement>(null);
  const postCommand = useCorrectionCommand(tenantId, event?.correction_id ?? "", csrfToken);

  if (!event)
    return (
      <>
        <PageHeader
          eyebrow="Controls / Correction record"
          title="Transfer correction"
          description="Loading the selected immutable control record."
        />
        {error ? (
          <StatePanel
            kind="error"
            title="Correction evidence unavailable"
            message={error}
          />
        ) : (
          <StatePanel
            title="Loading correction evidence"
            message="Authorization and linked journals are being verified."
          />
        )}
      </>
    );
  const requester = event.requester_subject_id === currentSubject;
  const canDecide = canApprove && !requester && event.status === "requested";
  const canPost = canApprove && !requester && event.status === "approved";
  const canCancel =
    canWrite &&
    requester &&
    (event.status === "requested" || event.status === "approved");
  const activeCorrectionId = event.correction_id;

  async function beginPostReview() {
    setRestorePostTrigger(true);
    setPostVerificationError(null);
    postCommand.setOutcome(null);
    setPostEvidence(null);
    setPostReviewOpen(true);
    setVerifyingPost(true);
    const current = await onRefresh();
    setVerifyingPost(false);
    if (current?.correction_id === activeCorrectionId
      && current.status === "approved"
      && current.requester_subject_id !== currentSubject
      && Boolean(current.approver_subject_id)) {
      setPostEvidence(current);
      return;
    }
    if (current?.status === "posted") postCommand.discard();
    setRestorePostTrigger(false);
    setPostVerificationError(current
      ? "This correction is no longer approved for posting by the current independent operator. Current evidence has been refreshed."
      : "LedgerSync could not verify the current approved correction. Posting remains disabled.");
    setPostReviewOpen(false);
    requestAnimationFrame(() => postOutcome.current?.focus());
  }

  async function confirmPost() {
    if (!postEvidence || postEvidence.status !== "approved") return;
    const result = await postCommand.send(postCommand.prepare());
    setRestorePostTrigger(false);
    setPostReviewOpen(false);
    if (result.kind === "success") await onRefresh();
    requestAnimationFrame(() => postOutcome.current?.focus());
  }

  return (
    <>
      <PageHeader
          eyebrow="Ledger / Correction"
        title="Correction control record"
          description="See the correction request, review decision, and any linked reverse transfer."
      />
      <section className="identity-strip">
        <div>
          <span>Correction ID</span>
          <CopyControl value={event.correction_id} />
        </div>
        <div>
          <span>Status</span>
          <StatusBadge tone={correctionTone(event.status)}>
            {event.status}
          </StatusBadge>
        </div>
        <div>
          <span>Policy</span>
          <strong>{event.policy_version}</strong>
        </div>
        <div>
          <span>Updated</span>
          <strong><Timestamp value={event.updated_at} /></strong>
        </div>
      </section>
      {verifiedAt && (
        <EvidenceFreshness
          state={
            error || !online ? "historical" : loading ? "refreshing" : "current"
          }
          verifiedAt={verifiedAt}
          label="Correction record"
          reason={
            error ?? (!online ? "Reconnect before taking action." : undefined)
          }
        />
      )}
      {error && (
        <StatePanel
          kind="error"
          title="Correction command or refresh failed"
          message={error}
        />
      )}
      {stepUpRequired && (
        <StatePanel
          kind="unknown"
          title="Recent authentication required"
          message="Reauthenticate with the approved identity provider, then return to this exact record. No correction command was assumed to succeed."
          action={
            <Link
              className="button primary"
              href={`/api/auth/sign-in?prompt=login&return_to=${encodeURIComponent(returnTo)}`}
            >
              Reauthenticate
            </Link>
          }
        />
      )}
      {(postVerificationError || postCommand.outcome) && (
        <div ref={postOutcome} tabIndex={-1}>
          {postVerificationError ? (
            <StatePanel kind="error" title="Posting evidence changed" message={postVerificationError} />
          ) : postCommand.outcome?.kind === "success" ? (
            <StatePanel title="Exact reverse transfer posted" message={`The authoritative correction confirms one additive reverse transfer. Request reference: ${postCommand.outcome.requestReference}.`} />
          ) : (
            <StatePanel
              kind={postCommand.outcome?.kind === "unknown" ? "unknown" : postCommand.outcome?.kind === "denied" ? "denied" : "error"}
              title={postCommand.outcome?.kind === "unknown" ? "Posting outcome unknown" : "Correction posting not completed"}
              message={postCommand.outcome?.message ?? "The posting was not completed."}
              action={postCommand.outcome?.kind === "unknown" ? <button className="button primary guarded-control" type="button" disabled={!online || postCommand.pending} onClick={() => void beginPostReview()}>Refresh before retry</button> : undefined}
            />
          )}
        </div>
      )}
      <section className="correction-evidence-grid">
        <article>
          <p className="eyebrow">Original · permanent</p>
          <h2>Committed transfer</h2>
          <dl className="evidence-list">
            <div>
              <dt>Transfer</dt>
              <dd>
                <Link href={`/transfers/${event.original_transfer_id}`}>
                  <CopyControl value={event.original_transfer_id} />
                </Link>
              </dd>
            </div>
            <div>
              <dt>Journal</dt>
              <dd>
                <CopyControl value={event.original_journal_id} />
              </dd>
            </div>
            <div>
              <dt>Route</dt>
              <dd>
                {event.debit_account_id} → {event.credit_account_id}
              </dd>
            </div>
            <div>
              <dt>Amount</dt>
              <dd><Money currency={event.currency} minorUnits={event.amount_minor} /></dd>
            </div>
          </dl>
        </article>
        <article
          className={event.compensation_transfer_id ? "posted" : "pending"}
        >
          <p className="eyebrow">Compensation · additive</p>
          <h2>
            {event.compensation_transfer_id
              ? "Posted reverse transfer"
              : "Not posted"}
          </h2>
          {event.compensation_transfer_id ? (
            <dl className="evidence-list">
              <div>
                <dt>Transfer</dt>
                <dd>
                  <Link href={`/transfers/${event.compensation_transfer_id}`}>
                    <CopyControl value={event.compensation_transfer_id} />
                  </Link>
                </dd>
              </div>
              <div>
                <dt>Journal</dt>
                <dd>
                  {event.compensation_journal_id ? (
                    <CopyControl value={event.compensation_journal_id} />
                  ) : (
                    "Unavailable"
                  )}
                </dd>
              </div>
              <div>
                <dt>Route</dt>
                <dd>
                  {event.credit_account_id} → {event.debit_account_id}
                </dd>
              </div>
              <div>
                <dt>Amount</dt>
                <dd><Money currency={event.currency} minorUnits={event.amount_minor} /></dd>
              </div>
            </dl>
          ) : (
            <p>
              The original ledger remains unchanged until an authorized approver
              posts the exact reversal.
            </p>
          )}
        </article>
      </section>
      <section className="surface correction-rationale">
        <div>
          <p className="eyebrow">Reasoned evidence</p>
          <h2>{label(event.reason_code)}</h2>
          <p>{event.operator_note}</p>
        </div>
        <dl>
          <div>
            <dt>Requester</dt>
            <dd>{event.requester_subject_id}</dd>
          </div>
          <div>
            <dt>Approver</dt>
            <dd>
              {event.approver_subject_id ?? "Independent approval pending"}
            </dd>
          </div>
          <div>
            <dt>Control mode</dt>
            <dd>{label(event.control_mode)}</dd>
          </div>
          <div>
            <dt>Approval expires</dt>
            <dd><Timestamp value={event.approval_expires_at} /></dd>
          </div>
          {event.decision_reason && (
            <div>
              <dt>Decision reason</dt>
              <dd>{event.decision_reason}</dd>
            </div>
          )}
        </dl>
      </section>
      <DisclosureSection
        id="correction-related-evidence"
        title="Lifecycle and related evidence"
        summary="Inspect authorized records linked to this correction after reviewing the original and requested reversal."
        lazy
      >
        <RelatedEvidenceRail sourceType="correction" sourceId={event.correction_id} />
      </DisclosureSection>
      {(canDecide || canPost || canCancel) && (
        <section className="correction-actions">
          <header>
            <div>
              <p className="eyebrow">Authorized next command</p>
              <h2>
                {canDecide
                  ? "Independent review"
                  : canPost
                    ? "Post exact compensation"
                    : "Cancel request"}
              </h2>
            </div>
            <StatusBadge tone="warning">step-up protected</StatusBadge>
          </header>
          {!canPost && (
            <FormField label="Decision or cancellation reason" requirement="required" hint="Explain why you approve, reject, or cancel this correction."><textarea
                rows={3}
                maxLength={500}
                required
                value={decisionReason}
                onChange={(change) => onReason(change.target.value)}
              /></FormField>
          )}
          <div>
            {canDecide && (
              <>
                <button
                  className="button primary"
                  type="button"
                  disabled={!online || actionBusy || !decisionReason.trim()}
                  onClick={() => onAction("approve")}
                >
                  Approve request
                </button>
                <button
                  className="button danger"
                  type="button"
                  disabled={!online || actionBusy || !decisionReason.trim()}
                  onClick={() => onAction("reject")}
                >
                  Reject request
                </button>
              </>
            )}
            {canPost && (
              <button
                ref={postTrigger}
                className="button danger guarded-control"
                type="button"
                disabled={!online || actionBusy || postCommand.pending}
                onClick={() => void beginPostReview()}
              >
                {verifyingPost ? "Verifying…" : postCommand.intent ? "Review same reverse-transfer retry" : "Review reverse-transfer posting"}
              </button>
            )}
            {canCancel && (
              <button
                className="button secondary"
                type="button"
                disabled={!online || actionBusy || !decisionReason.trim()}
                onClick={() => onAction("cancel")}
              >
                Cancel request
              </button>
            )}
            <button
              className="button secondary"
              type="button"
              disabled={!online || loading}
              onClick={onRefresh}
            >
              Refresh record
            </button>
          </div>
        </section>
      )}
      {requester &&
        canApprove &&
        (event.status === "requested" || event.status === "approved") && (
          <StatePanel
            kind="denied"
            title="Requester cannot approve or post"
            message="Dual control requires a different authorized subject. This UI hides the command, and the API enforces the same separation."
          />
        )}
      <Link className="text-link back-link" href={backHref}>
        ← {backHref.startsWith("/approvals") ? "Back to approvals" : "Back to correction queue"}
      </Link>
      <ConfirmationDialog
        open={postReviewOpen}
        eyebrow="Permanent additive correction"
        title="Post exact reverse transfer?"
        description="LedgerSync refreshed this correction before review. The original journal remains unchanged; confirmation creates one new balanced reversal."
        confirmLabel={postCommand.intent ? "Retry same reverse transfer" : "Post exact reverse transfer"}
        busyLabel="Posting…"
        className="financial-command-dialog"
        busy={postCommand.pending || verifyingPost}
        confirmDisabled={!postEvidence || postEvidence.status !== "approved"}
        returnFocusRef={postTrigger}
        restoreTriggerFocus={restorePostTrigger}
        onDismiss={() => setPostReviewOpen(false)}
        onConfirm={() => void confirmPost()}
      >
        {verifyingPost ? (
          <StatePanel title="Verifying current approval" message="Posting remains disabled until the authoritative correction and independent approval are available." />
        ) : postEvidence && (
          <dl className="review-grid">
            <div><dt>Permanent effect</dt><dd>Create one additive balanced reverse transfer</dd></div>
            <div><dt>Exact amount</dt><dd><Money currency={postEvidence.currency} minorUnits={postEvidence.amount_minor} /></dd></div>
            <div><dt>Reverse route</dt><dd><code>{postEvidence.credit_account_id}</code> → <code>{postEvidence.debit_account_id}</code></dd></div>
            <div><dt>Original transfer</dt><dd><code>{postEvidence.original_transfer_id}</code></dd></div>
            <div><dt>Independent approver</dt><dd>{postEvidence.approver_subject_id ?? "Approval evidence unavailable"}</dd></div>
            <div><dt>Approval expires</dt><dd><Timestamp value={postEvidence.approval_expires_at} /></dd></div>
          </dl>
        )}
      </ConfirmationDialog>
    </>
  );
}
