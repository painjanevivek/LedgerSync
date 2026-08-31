"use client";

import { ArrowsCounterClockwise, WarningCircle } from "@phosphor-icons/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

import { ConsoleFooter, ConsoleShell } from "@/features/console/ConsoleShell";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { CopyControl } from "@/ui/controls/CopyControl.client";
import { FocusedRetry } from "@/ui/controls/FocusedRetry.client";
import { DataTableRegion } from "@/ui/display/DataTableRegion";
import { EvidenceFreshness } from "@/ui/display/Evidence";
import { PageHeader } from "@/ui/display/PageHeader";
import { StatePanel } from "@/ui/display/StatePanel";
import { StatusBadge } from "@/ui/display/StatusBadge";
import { FormField } from "@/ui/forms/FormField.client";
import type {
  CorrectionPage,
  CorrectionFilters,
  CorrectionStatus,
  CorrectionSubmission,
  TransferCorrection,
} from "@/lib/api/corrections";
import { readJSON, unavailableMessage } from "@/lib/api/client";
import { Money } from "@/ui/display/Money";
import { Timestamp } from "@/ui/display/Timestamp";
import { useCorrectionCommand } from "@/features/corrections/useCorrectionCommand";
import { ConfirmationDialog } from "@/ui/overlays/ConfirmationDialog.client";
import { beginEvidenceRequest, createEvidenceRequestCoordinator, finishEvidenceRequest, invalidateEvidenceRequests, isEvidenceRequestCurrent } from "@/features/console/evidenceRequestCoordinator";
import { correctionsURL } from "@/lib/page-query/corrections";
import { RelatedEvidenceRail } from "@/features/investigation/RelatedEvidenceRail";

const statusOptions: ReadonlyArray<CorrectionStatus | "all"> = [
  "all",
  "requested",
  "approved",
  "posted",
  "rejected",
  "cancelled",
  "expired",
];
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

export function CorrectionsConsole({
  correctionId,
  detailReturnTo,
  filters = { status: "" },
  invalidQuery = false,
}: Readonly<{ correctionId?: string; detailReturnTo?: string; filters?: CorrectionFilters; invalidQuery?: boolean }>) {
  const router = useRouter();
  const { session, sessionLoading, sessionError, online, hasScope } = useConsoleSession();
  const [events, setEvents] = useState<TransferCorrection[]>([]);
  const [selected, setSelected] = useState<TransferCorrection | null>(null);
  const [nextCursor, setNextCursor] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [actionBusy, setActionBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [verifiedAt, setVerifiedAt] = useState<string>();
  const [decisionReason, setDecisionReason] = useState("");
  const [stepUpRequired, setStepUpRequired] = useState(false);
  const listRequests = useRef(createEvidenceRequestCoordinator());
  const detailRequests = useRef(createEvidenceRequestCoordinator());

  const loadList = useCallback(
    async () => {
      const query = new URLSearchParams({ limit: "25" });
      if (filters.status) query.set("status", filters.status);
      if (filters.cursor) query.set("cursor", filters.cursor);
      const request = beginEvidenceRequest(listRequests.current, `corrections:list:${query}`, "replace");
      if (!request) return;
      if (!request.sameResource) {
        setEvents([]);
        setNextCursor(undefined);
        setVerifiedAt(undefined);
      }
      setLoading(true);
      setError(null);
      const response = await readJSON<CorrectionPage>(
        `/api/transfer-corrections?${query}`,
      );
      if (!isEvidenceRequestCurrent(listRequests.current, request.token)) return;
      if (response.ok && Array.isArray(response.data.events)) {
        setEvents(response.data.events);
        setNextCursor(response.data.next_cursor || undefined);
        setVerifiedAt(new Date().toISOString());
      } else
        setError(
          unavailableMessage(
            response.status,
            "transfer correction evidence",
            response.requestReference,
          ),
        );
      if (finishEvidenceRequest(listRequests.current, request.token)) setLoading(false);
    },
    [filters.cursor, filters.status],
  );

  const loadEvent = useCallback(async () => {
    if (!correctionId) return null;
    const request = beginEvidenceRequest(detailRequests.current, `correction:${correctionId}`);
    if (!request) return null;
    if (!request.sameResource) { setSelected(null); setVerifiedAt(undefined); }
    setLoading(true);
    setError(null);
    const response = await readJSON<TransferCorrection>(
      `/api/transfer-corrections/${encodeURIComponent(correctionId)}`,
    );
    if (!isEvidenceRequestCurrent(detailRequests.current, request.token)) return null;
    if (response.ok && response.data.correction_id) {
      setSelected(response.data);
      setVerifiedAt(new Date().toISOString());
      setLoading(false);
      return response.data;
    } else {
      setError(
        response.status === 404
          ? `The correction record was not found in this authorized tenant scope. Request reference: ${response.requestReference}.`
          : unavailableMessage(
              response.status,
              "transfer correction evidence",
              response.requestReference,
            ),
      );
    }
    if (finishEvidenceRequest(detailRequests.current, request.token)) setLoading(false);
    return null;
  }, [correctionId]);

  useEffect(() => {
    if (!session || !online || !hasScope("corrections:read") || invalidQuery)
      return;
    const listCoordinator = listRequests.current;
    const detailCoordinator = detailRequests.current;
    const timer = window.setTimeout(
      () => void (correctionId ? loadEvent() : loadList()),
      0,
    );
    return () => {
      window.clearTimeout(timer);
      invalidateEvidenceRequests(listCoordinator);
      invalidateEvidenceRequests(detailCoordinator);
    };
  }, [correctionId, hasScope, invalidQuery, loadEvent, loadList, online, session]);
  async function act(action: "approve" | "reject" | "cancel") {
    if (!session || !selected) return;
    setActionBusy(true);
    setError(null);
    setStepUpRequired(false);
    try {
      const headers: Record<string, string> = {
        "X-CSRF-Token": session.csrf_token,
        "Content-Type": "application/json",
      };
      const response = await fetch(
        `/api/transfer-corrections/${encodeURIComponent(selected.correction_id)}/${action}`,
        {
          method: "POST",
          headers,
          body: JSON.stringify({ reason: decisionReason }),
        },
      );
      const payload = (await response.json().catch(() => ({}))) as
        | TransferCorrection
        | CorrectionSubmission
        | { error?: { code?: string } };
      if (!response.ok) {
        const code = "error" in payload ? payload.error?.code : undefined;
        if (response.status === 428 || code === "step_up_required")
          setStepUpRequired(true);
        throw new Error(
          response.status === 504
            ? "The command outcome is unknown. Refresh this record before any retry."
            : code === "step_up_required"
              ? "Recent authentication is required before this command can be authorized."
              : `The correction command was not recorded (${code ?? response.status}).`,
        );
      }
      const event =
        "event" in payload ? payload.event : (payload as TransferCorrection);
      if (event.correction_id) setSelected(event);
      else await loadEvent();
      setDecisionReason("");
      setVerifiedAt(new Date().toISOString());
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "The correction command could not be verified.",
      );
    } finally {
      setActionBusy(false);
    }
  }

  if (sessionLoading)
    return (
      <ConsoleShell
        section="corrections"
        tenantLabel="Verifying tenant"
        tenantMeta="Secure session"
        environmentLabel="Checking environment"
        operatorLabel="Verifying operator"
        operatorMeta="Authorization pending"
      >
        <PageHeader
          eyebrow="Controls · LedgerSync"
          title="Verifying access"
          description="Checking correction evidence scopes before any control record is shown."
        />
        <StatePanel
          title="Loading authorized evidence"
          message="No correction status is inferred while the session boundary is verified."
        />
        <ConsoleFooter />
      </ConsoleShell>
    );
  if (!session)
    return (
      <main className="boot-screen">
        <p className="eyebrow">Access not verified</p>
        <h1>Correction workspace unavailable</h1>
        <StatePanel
          kind={sessionError ? "error" : "denied"}
          title={
            sessionError
              ? "Session evidence unavailable"
              : "No authorized session"
          }
          message={
            sessionError ??
            "Sign in with an approved finance operator identity. No correction evidence is displayed."
          }
        />
      </main>
    );
  const canRead = hasScope("corrections:read");
  const canWrite = hasScope("corrections:write");
  const canApprove = hasScope("corrections:approve");
  const returnTo = correctionId
    ? `/corrections/${encodeURIComponent(correctionId)}`
    : "/corrections";
  return (
    <ConsoleShell
      section="corrections"
      tenantLabel={session.tenant_label ?? "Ledger tenant"}
      tenantMeta={session.tenant_id}
      environmentLabel={
        session.environment === "local" ? "Local workspace" : "Verified production"
      }
      operatorLabel={session.operator_label ?? session.subject_id}
      operatorMeta="Authorized control operator"
    >
      {!online && (
        <div className="offline-banner" role="status">
          <WarningCircle weight="fill" aria-hidden="true" />
          <span>
            <strong>You are offline.</strong> Correction commands are disabled;
            retained evidence is historical.
          </span>
        </div>
      )}
      {!canRead ? (
        <>
          <PageHeader
            eyebrow="Controls / Immutable corrections"
            title="Transfer corrections"
            description="Additive reversal evidence with independent authorization."
          />
          <StatePanel
            kind="denied"
            title="Correction read scope required"
            message="Ask a tenant administrator for corrections:read. LedgerSync does not broaden control evidence visibility."
          />
        </>
      ) : invalidQuery && !correctionId ? (
        <>
          <PageHeader eyebrow="Ledger / Corrections" title="Transfer corrections" description="Review and correct a transfer without changing the original record." />
          <StatePanel kind="error" title="Invalid correction investigation URL" message="The shared URL contains an unknown, repeated, empty, oversized, or malformed filter. No protected correction request was made." action={<button className="button secondary" type="button" onClick={() => router.replace("/corrections")}>Clear invalid filters</button>} />
        </>
      ) : correctionId ? (
        <CorrectionDetail
          event={selected}
          loading={loading}
          error={error}
          verifiedAt={verifiedAt}
          online={online}
          currentSubject={session.subject_id}
          tenantId={session.tenant_id}
          csrfToken={session.csrf_token}
          canWrite={canWrite}
          canApprove={canApprove}
          actionBusy={actionBusy}
          decisionReason={decisionReason}
          stepUpRequired={stepUpRequired}
          returnTo={returnTo}
          backHref={detailReturnTo ?? "/corrections"}
          onReason={setDecisionReason}
          onRefresh={loadEvent}
          onAction={(action) => void act(action)}
        />
      ) : (
        <CorrectionList
          key={correctionsURL(filters)}
          events={events}
          loading={loading}
          error={error}
          verifiedAt={verifiedAt}
          online={online}
          filters={filters}
          nextCursor={nextCursor}
          onApplyFilters={(next) => router.push(correctionsURL(next))}
          onClearFilters={() => router.push("/corrections")}
          onRefresh={() => void loadList()}
        />
      )}
      <ConsoleFooter />
    </ConsoleShell>
  );
}

function CorrectionList({
  events,
  loading,
  error,
  verifiedAt,
  online,
  filters,
  nextCursor,
  onApplyFilters,
  onClearFilters,
  onRefresh,
}: Readonly<{
  events: TransferCorrection[];
  loading: boolean;
  error: string | null;
  verifiedAt?: string;
  online: boolean;
  filters: CorrectionFilters;
  nextCursor?: string;
  onApplyFilters: (filters: CorrectionFilters) => void;
  onClearFilters: () => void;
  onRefresh: () => void;
}>) {
  const [draftStatus, setDraftStatus] = useState<CorrectionFilters["status"]>(filters.status);
  const returnTo = correctionsURL(filters);
  const nextHref = nextCursor ? correctionsURL({ ...filters, cursor: nextCursor }) : undefined;
  return (
    <>
      <PageHeader
        eyebrow="Ledger / Corrections"
        title="Transfer corrections"
        description="Review and correct a transfer without changing the original record."
      />
      <section className="correction-boundary">
        <ArrowsCounterClockwise aria-hidden="true" />
        <div>
          <p className="eyebrow">Immutable correction rule</p>
          <h2>Original evidence stays permanent</h2>
          <p>
            An approved correction may post one exact reverse transfer.
            Requester and approver remain distinct, and both journals stay
            linked.
          </p>
        </div>
        <strong>Policy-bound</strong>
      </section>
      <form className="surface list-filter-bar" onSubmit={(event) => { event.preventDefault(); onApplyFilters({ status: draftStatus }); }}>
        <FormField label="Exact correction status" requirement="optional" hint="The server filters the complete correction history before pagination."><select
            value={draftStatus}
            onChange={(event) =>
              setDraftStatus(event.target.value as CorrectionFilters["status"])
            }
          >
            {statusOptions.map((value) => (
              <option key={value} value={value === "all" ? "" : value}>
                {value === "all" ? "All statuses" : label(value)}
              </option>
            ))}
          </select></FormField>
        <div className="action-row"><button className="button primary" type="submit" disabled={loading}>Apply filters</button><button className="button secondary" type="button" disabled={loading} onClick={onClearFilters}>Clear all</button><button className="button secondary" type="button" disabled={!online || loading} onClick={onRefresh}>Refresh evidence</button></div>
      </form>
      {error && (
        <StatePanel
          kind="error"
          title="Correction evidence unavailable"
          message={error}
          action={<FocusedRetry label="Retry correction evidence" onRetry={onRefresh} disabled={!online} busy={loading} />}
        />
      )}
      {verifiedAt && events.length > 0 && (
        <EvidenceFreshness
          state={
            error || !online ? "historical" : loading ? "refreshing" : "current"
          }
          verifiedAt={verifiedAt}
          label="Correction queue"
          reason={
            error ??
            (!online
              ? "Reconnect before treating this queue as current."
              : undefined)
          }
        />
      )}
      {loading && events.length === 0 ? (
        <StatePanel
          title="Loading correction evidence"
          message="Policy, authorization, and journal linkage are being verified."
        />
      ) : !error && events.length === 0 ? (
        <StatePanel
          title="No corrections found"
          message="No correction records match this authorized status scope."
        />
      ) : (
        events.length > 0 && (
          <section className="ledger-section" aria-busy={loading}>
            <div className="section-heading"><div><p className="eyebrow">Newest requested evidence first</p><h2>Correction history</h2><p>{events.length} record{events.length === 1 ? "" : "s"} on this page. A total is not calculated or implied.</p></div></div>
            <DataTableRegion label="Transfer correction queue">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Correction</th>
                    <th>Status</th>
                    <th>Exact amount</th>
                    <th>Reason</th>
                    <th>Expires UTC</th>
                    <th>Action</th>
                  </tr>
                </thead>
                <tbody>
                  {events.map((event) => (
                    <tr key={event.correction_id}>
                      <td>
                        <CopyControl value={event.correction_id} />
                      </td>
                      <td>
                        <StatusBadge tone={correctionTone(event.status)}>
                          {event.status}
                        </StatusBadge>
                      </td>
                      <td className="number-cell">
                        <Money currency={event.currency} minorUnits={event.amount_minor} />
                      </td>
                      <td>{label(event.reason_code)}</td>
                      <td><Timestamp value={event.approval_expires_at} /></td>
                      <td>
                        <Link
                          className="record-link"
                          href={`/corrections/${encodeURIComponent(event.correction_id)}?return_to=${encodeURIComponent(returnTo)}`}
                        >
                          Open control record
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </DataTableRegion>
            <div className="pagination"><span>{nextHref ? "More matching correction records are available" : "End of this filtered correction history"}</span>{nextHref ? <Link className="button secondary" href={nextHref}>Next page</Link> : <button className="button secondary" type="button" disabled>Next page</button>}</div>
          </section>
        )
      )}
    </>
  );
}

function CorrectionDetail({
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
      <RelatedEvidenceRail sourceType="correction" sourceId={event.correction_id} />
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
