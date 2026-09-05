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
import { SavedViewCapture } from "@/features/investigation/SavedViewCapture";
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
import { beginEvidenceRequest, createEvidenceRequestCoordinator, finishEvidenceRequest, invalidateEvidenceRequests, isEvidenceRequestCurrent } from "@/features/console/evidenceRequestCoordinator";
import { correctionsURL } from "@/lib/page-query/corrections";
import { useExperienceMode } from "@/features/console/ExperienceModeBoundary";
import { CorrectionDetail } from "@/features/corrections/CorrectionDetail";
import { RecordIdentity } from "@/ui/presentation/RecordIdentity";
import { RelativeTime } from "@/ui/presentation/RelativeTime";
import { TechnicalDetails } from "@/ui/presentation/TechnicalDetails";

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

function correctionStatusLabel(status: CorrectionStatus): string {
  if (status === "requested") return "Waiting for review";
  if (status === "approved") return "Ready to correct";
  if (status === "posted") return "Correction completed";
  if (status === "rejected") return "Correction not approved";
  if (status === "cancelled") return "Correction cancelled";
  return "Review period ended";
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
  const { mode } = useExperienceMode();
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
      {mode === "expert" && <form className="surface list-filter-bar" onSubmit={(event) => { event.preventDefault(); onApplyFilters({ status: draftStatus }); }}>
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
      </form>}
      {mode === "expert" && <SavedViewCapture domain="corrections" filters={{ status: filters.status || undefined }} />}
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
        events.length > 0 && mode === "simple" ? (
          <section className="simple-record-section" aria-busy={loading}><div className="simple-section-heading"><div><h2>Corrections</h2><p>{events.length} {events.length === 1 ? "record needs" : "records need"} your attention on this page.</p></div></div><ul className="simple-record-list">{events.map((event) => <li key={event.correction_id} data-tone={event.status === "posted" ? "positive" : ["rejected", "cancelled", "expired"].includes(event.status) ? "danger" : "warning"}><article><div className="simple-record-main"><div><strong>{correctionStatusLabel(event.status)}</strong><span>{label(event.reason_code)} · <RelativeTime value={event.requested_at} /></span></div><Money currency={event.currency} minorUnits={event.amount_minor} /></div><p>The original transfer stays unchanged. Any approved correction creates a linked reverse transfer.</p><div className="simple-record-actions"><Link className="button secondary" href={`/corrections/${encodeURIComponent(event.correction_id)}?return_to=${encodeURIComponent(returnTo)}`}>{event.status === "requested" ? "Review correction" : "View correction"}</Link><TechnicalDetails><RecordIdentity label="Correction reference" value={event.correction_id} /><p>Approval expires <Timestamp value={event.approval_expires_at} />.</p></TechnicalDetails></div></article></li>)}</ul>{nextHref && <div className="pagination"><span>More corrections are available</span><Link className="button secondary" href={nextHref}>Next page</Link></div>}</section>
        ) : events.length > 0 && (
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
