"use client";

import { CheckCircle, Clock, Equals, LinkSimple, WarningCircle } from "@phosphor-icons/react";
import Link from "next/link";
import { FormEvent, useState, type ReactNode } from "react";

import type { Account, ConsoleSession } from "@/features/accounts/types";
import { CopyControl, DataTableRegion, EvidenceFreshness, FocusedRetry, PageHeader, Pagination, StatePanel, StatusBadge } from "@/features/console/components";
import { accountLabel, utcDateTime } from "@/features/console/format";
import type { FundingEvent, FundingReconciliation, FundingStatus } from "@/lib/api/funding";
import { formatMinorUnits } from "@/lib/money";

function fundingTone(status: FundingStatus): "success" | "warning" | "danger" | "neutral" | "info" {
  if (status === "posted" || status === "compensated") return "success";
  if (status === "rejected") return "danger";
  if (status === "requested") return "warning";
  return "info";
}

export function FundingWorkspaceRail({
  events,
  event,
  loading,
  error,
  online,
}: Readonly<{
  events: FundingEvent[];
  event?: FundingEvent | null;
  loading: boolean;
  error: string | null;
  online: boolean;
}>) {
  const record = event ?? events[0];
  const reviewSummary = loading
    ? "Refreshing the authorized funding page. A missing result is never inferred while this check is running."
    : error
      ? "The latest check did not complete. Keep any loaded record historical until it can be refreshed."
      : record
        ? `${events.length || 1} authorized funding ${events.length === 1 ? "record is" : "records are"} in the current review scope. The latest status is ${record.status}.`
        : "No funding records are in the current authorized scope.";
  const nextSafeAction = !online
    ? "Reconnect before recording, deciding, or posting a funding record."
    : record?.status === "requested"
      ? "Independently match the external reference and supporting location before an approver records a decision."
      : record?.status === "approved"
        ? "Post only after the approved record still matches the external reference and exact amount."
        : record?.status === "posted" || record?.status === "compensated"
          ? "Use the record history to inspect the immutable decision and balanced journal."
          : "Record an external reference and controlled supporting location before it can enter ledger review.";

  return (
    <div className="funding-context-rail-content">
      <section className="funding-context-card">
        <p className="eyebrow">Funding boundary</p>
        <h2>Reference, not custody</h2>
        <p>
          LedgerSync records customer-authorized external value references. It
          does not describe these records as deposits, bank settlement, or
          custody.
        </p>
      </section>
      <section className="funding-context-card">
        <p className="eyebrow">Approval requirement</p>
        <h2>Independent review protects posting</h2>
        <p>
          A funding record is reviewable first. In production, a separate
          finance operator must approve it before a balanced journal can post.
        </p>
      </section>
      <section className="funding-context-card">
        <p className="eyebrow">Exact money</p>
        <h2>Minor units stay authoritative</h2>
        <p>
          Amounts are stored as exact minor units. A record does not change a
          balance until its approved journal is posted.
        </p>
      </section>
      <section className="funding-context-card funding-context-current">
        <p className="eyebrow">Current review</p>
        <h2>Next safe action</h2>
        <p>{reviewSummary}</p>
        <p className="funding-context-next">{nextSafeAction}</p>
      </section>
    </div>
  );
}

export function FundingListView({ events, accounts, nextCursor, verifiedAt, loading, error, online, canWrite, onOpenRequest, onRefresh, onNext }: Readonly<{
  events: FundingEvent[]; accounts: Account[]; nextCursor?: string; verifiedAt?: string; loading: boolean; error: string | null; online: boolean; canWrite: boolean;
  onOpenRequest: () => void; onRefresh: () => void; onNext: () => void;
}>) {
  const labels = new Map(accounts.map((account) => [account.account_id, accountLabel(account)]));
  return <>
    <PageHeader eyebrow="Ledger / Funding records" title="Funding records" description="Recorded external value references, independent decisions, and the balanced journals they authorize.">
      <div className="header-actions"><button className="button secondary" type="button" disabled={!online || loading} onClick={onRefresh}>Refresh records</button>{canWrite && <button className="button primary guarded-control" type="button" onClick={onOpenRequest}>Record funding</button>}</div>
    </PageHeader>
    {verifiedAt && events.length > 0 && <EvidenceFreshness state={error || !online ? "historical" : loading ? "refreshing" : "current"} verifiedAt={verifiedAt} label="Funding records" reason={error ?? (!online ? "Reconnect before acting on these records." : undefined)} />}
    {error && <StatePanel kind="error" title="Funding records unavailable" message={error} action={<FocusedRetry label="Retry funding records" onRetry={onRefresh} disabled={!online} busy={loading} />} />}
    {loading && events.length === 0 ? <StatePanel title="Loading funding records" message="LedgerSync is verifying one bounded tenant page; an empty result is not inferred while the request is pending." /> : events.length === 0 && !error ? <StatePanel title="No funding records yet" message="Record an external reference when value is confirmed outside LedgerSync and must enter the internal ledger under controlled review." action={canWrite ? <button className="button primary" type="button" onClick={onOpenRequest}>Record first funding</button> : undefined} /> : events.length > 0 && <section className="ledger-section funding-ledger" aria-labelledby="funding-ledger-heading" aria-busy={loading}>
      <div className="section-heading"><div><p className="eyebrow">Immutable tenant record</p><h2 id="funding-ledger-heading">Funding history</h2><p>Newest requests first. Each row remains inspectable after rejection, posting, or compensation.</p></div></div>
      <DataTableRegion label="Funding record comparison"><table className="data-table"><thead><tr><th>External reference</th><th>Destination</th><th>Exact amount</th><th>State</th><th>Recorded</th><th>Action</th></tr></thead><tbody>{events.map((event) => <tr key={event.funding_event_id}><td><strong>{event.compensation_of_event_id ? "Compensation" : "Funding"}</strong><code>{event.external_reference}</code></td><td><strong>{labels.get(event.destination_account_id) ?? "Authorized account"}</strong><code>{event.destination_account_id}</code></td><td className="number-cell">{formatMinorUnits(event.currency, event.amount_minor)}</td><td><StatusBadge tone={fundingTone(event.status)}>{event.status}</StatusBadge>{event.demo_policy && <small>Local single-operator policy</small>}</td><td><time>{utcDateTime(event.requested_at)}</time><small>By {event.requester_subject_id}</small></td><td><Link className="record-link" href={`/funding/${encodeURIComponent(event.funding_event_id)}`}>Open record <span aria-hidden="true">→</span></Link></td></tr>)}</tbody></table></DataTableRegion>
      <Pagination nextCursor={nextCursor} busy={loading} onNext={onNext} label="Next records page" />
    </section>}
  </>;
}

export function FundingDetailView({ event, account, session, reconciliation, verifiedAt, loading, actionBusy, error, online, canWrite, canApprove, onRefresh, onAction, onReconcile }: Readonly<{
  event: FundingEvent | null; account?: Account; session: ConsoleSession; reconciliation: FundingReconciliation | null; verifiedAt?: string; loading: boolean; actionBusy: boolean; error: string | null; online: boolean; canWrite: boolean; canApprove: boolean;
  onRefresh: () => void; onAction: (path: string, body?: Record<string, string>, idempotencyKey?: string) => Promise<boolean>; onReconcile: () => void;
}>) {
  if (!event) return <><PageHeader eyebrow="Ledger / Funding records" title="Funding detail" description="Verifying the selected funding record and its journal links." />{error ? <StatePanel kind="error" title="Funding record unavailable" message={error} action={<FocusedRetry label="Retry this record" onRetry={onRefresh} disabled={!online} busy={loading} />} /> : <StatePanel title="Loading funding record" message="No decision or journal state is inferred until the selected record is verified." />}</>;
  const reviewComplete = ["approved", "posted", "compensated"].includes(event.status);
  const journalComplete = ["posted", "compensated"].includes(event.status);
  return <>
    <PageHeader eyebrow="Ledger / Funding records" title={event.compensation_of_event_id ? "Compensation record" : "Funding record"} description="One immutable external reference, its decision trail, and its exact balanced journal.">
      <div className="header-actions"><button className="button secondary" type="button" disabled={!online || loading} onClick={onRefresh}>Refresh record</button><Link className="button secondary" href="/funding">Back to funding</Link></div>
    </PageHeader>
    {event.demo_policy && <div className="funding-demo-banner"><WarningCircle weight="fill" aria-hidden="true" /><div><strong>Single-operator local policy</strong><p>This label is server-owned. Production requires a different finance operator to approve the requester’s record.</p></div></div>}
    {verifiedAt && <EvidenceFreshness state={error || !online ? "historical" : loading ? "refreshing" : "current"} verifiedAt={verifiedAt} label="Funding event" reason={error ?? (!online ? "Reconnect before making a financial decision." : undefined)} />}
    {error && <StatePanel kind="error" title="Refresh failed" message={error} />}
    <section className="funding-docket" aria-labelledby="funding-docket-heading">
      <header><div><p className="eyebrow">Funding record</p><h2 id="funding-docket-heading">{event.external_reference}</h2></div><StatusBadge tone={fundingTone(event.status)}>{event.status}</StatusBadge></header>
      <div className="funding-amount-line"><span>Exact recorded amount</span><strong>{formatMinorUnits(event.currency, event.amount_minor)}</strong><small>Destination · {account ? accountLabel(account) : event.destination_account_id}</small></div>
      <ol className="funding-evidence-rail">
        <FundingStage sequence="01" state="complete" title="External reference recorded" meta={utcDateTime(event.requested_at)}><dl><div><dt>Supporting reference</dt><dd>{event.evidence_reference}</dd></div><div><dt>Requester</dt><dd>{event.requester_subject_id}</dd></div><div><dt>Event ID</dt><dd><CopyControl value={event.funding_event_id} /></dd></div></dl></FundingStage>
        <FundingStage sequence="02" state={event.status === "rejected" ? "rejected" : reviewComplete ? "complete" : "current"} title={event.status === "rejected" ? "Request rejected" : reviewComplete ? "Independent decision recorded" : "Independent decision required"} meta={event.approver_subject_id ? `By ${event.approver_subject_id}` : "Awaiting finance review"}><dl><div><dt>Policy</dt><dd>{event.demo_policy ? "Local workspace · single operator" : "Production · dual control"}</dd></div>{event.decision_reason && <div><dt>Decision reason</dt><dd>{event.decision_reason}</dd></div>}</dl></FundingStage>
        <FundingStage sequence="03" state={journalComplete ? "complete" : event.status === "rejected" ? "blocked" : reviewComplete ? "current" : "blocked"} title={journalComplete ? "Balanced journal posted" : "Journal not posted"} meta={event.journal_transaction_id ? `Journal ${event.journal_transaction_id}` : "Balances unchanged"}><dl><div><dt>Destination version</dt><dd>{event.balance_version ? `v${event.balance_version}` : "Not available before posting"}</dd></div><div><dt>Clearing identity</dt><dd>{event.system_account_id ? <CopyControl value={event.system_account_id} label="Copy funding clearing account ID" /> : "Pending"}</dd></div></dl></FundingStage>
      </ol>
      {event.compensation_of_event_id && <div className="funding-linked-record"><LinkSimple aria-hidden="true" /><p><strong>Compensates original record</strong><Link href={`/funding/${encodeURIComponent(event.compensation_of_event_id)}`}>{event.compensation_of_event_id}</Link></p></div>}
      {event.compensation_event_id && <div className="funding-linked-record"><LinkSimple aria-hidden="true" /><p><strong>Preserved with additive compensation</strong><Link href={`/funding/${encodeURIComponent(event.compensation_event_id)}`}>{event.compensation_event_id}</Link></p></div>}
    </section>
    <FundingActionPanel event={event} session={session} online={online} busy={actionBusy} canWrite={canWrite} canApprove={canApprove} onAction={onAction} onReconcile={onReconcile} />
    {reconciliation && <section className={`funding-reconciliation ${reconciliation.status}`} aria-labelledby="funding-reconciliation-heading"><Equals aria-hidden="true" /><div><p className="eyebrow">External reference reconciliation</p><h2 id="funding-reconciliation-heading">{reconciliation.status === "matched" ? "Journal totals match" : "Journal mismatch requires investigation"}</h2><p>Expected {formatMinorUnits(reconciliation.currency, reconciliation.expected_minor)} · debit {formatMinorUnits(reconciliation.currency, reconciliation.posted_debit_minor)} · credit {formatMinorUnits(reconciliation.currency, reconciliation.posted_credit_minor)}</p><small>Checked {utcDateTime(reconciliation.checked_at)}</small></div></section>}
  </>;
}

function FundingStage({ sequence, state, title, meta, children }: Readonly<{ sequence: string; state: "complete" | "current" | "blocked" | "rejected"; title: string; meta: string; children: ReactNode }>) {
  return <li className={`funding-stage ${state}`}><div className="funding-stage-index">{state === "complete" ? <CheckCircle weight="fill" aria-hidden="true" /> : <span>{sequence}</span>}</div><article><header><div><h3>{title}</h3><p>{meta}</p></div>{state === "current" && <Clock aria-hidden="true" />}</header>{children}</article></li>;
}

function FundingActionPanel({ event, session, online, busy, canWrite, canApprove, onAction, onReconcile }: Readonly<{ event: FundingEvent; session: ConsoleSession; online: boolean; busy: boolean; canWrite: boolean; canApprove: boolean; onAction: (path: string, body?: Record<string, string>, idempotencyKey?: string) => Promise<boolean>; onReconcile: () => void }>) {
  const [decision, setDecision] = useState<"approve" | "reject" | null>(null);
  const [reason, setReason] = useState("");
  const [compensating, setCompensating] = useState(false);
  const [reasonCode, setReasonCode] = useState("external_evidence_reversed");
  const [operatorNote, setOperatorNote] = useState("");
  const [compensationIdempotencyKey, setCompensationIdempotencyKey] = useState<string>();
  const selfApprovalBlocked = session.environment !== "local" && event.requester_subject_id === session.subject_id;

  async function submitDecision(action: "approve" | "reject") {
    if (!reason.trim()) return;
    if (await onAction(action, { reason: reason.trim() })) { setDecision(null); setReason(""); }
  }
  async function submitCompensation(eventObject: FormEvent<HTMLFormElement>) {
    eventObject.preventDefault();
    const idempotencyKey = compensationIdempotencyKey ?? crypto.randomUUID();
    setCompensationIdempotencyKey(idempotencyKey);
    if (await onAction("compensations", { reasonCode, operatorNote: operatorNote.trim() }, idempotencyKey)) {
      setCompensating(false);
      setOperatorNote("");
      setCompensationIdempotencyKey(undefined);
    }
  }

  return <section className="funding-actions" aria-labelledby="funding-actions-heading"><header><div><p className="eyebrow">Permitted next step</p><h2 id="funding-actions-heading">Act on this funding record</h2></div><span>State · {event.status}</span></header>
    {event.status === "requested" && !decision && <div className="funding-action-choice"><p>{selfApprovalBlocked ? "A different production finance operator must decide this request." : "Approve only after independently matching the external reference and evidence location."}</p><div>{canApprove && !selfApprovalBlocked && <button className="button primary guarded-control" type="button" disabled={!online || busy} onClick={() => setDecision("approve")}>Review approval</button>}{canApprove && <button className="button secondary guarded-control" type="button" disabled={!online || busy} onClick={() => setDecision("reject")}>Review rejection</button>}</div></div>}
    {event.status === "requested" && decision && <form className="funding-decision-form" onSubmit={(formEvent) => { formEvent.preventDefault(); void submitDecision(decision); }}><label>{decision === "approve" ? "Approval reason" : "Rejection reason"}<textarea required maxLength={500} rows={3} value={reason} onChange={(change) => setReason(change.target.value)} placeholder="State the evidence checked and decision basis" /></label><div><button className="button secondary" type="button" disabled={busy} onClick={() => { setDecision(null); setReason(""); }}>Cancel</button><button className={decision === "approve" ? "button primary guarded-control" : "button danger guarded-control"} type="submit" disabled={!online || busy || !reason.trim()}>{busy ? "Recording…" : decision === "approve" ? "Approve evidence" : "Reject evidence"}</button></div></form>}
    {event.status === "approved" && <div className="funding-action-choice"><p>The decision is durable. Posting creates one balanced journal and updates the destination balance atomically.</p>{canWrite && <button className="button primary guarded-control" type="button" disabled={!online || busy} onClick={() => void onAction("post")}>{busy ? "Posting…" : "Post balanced journal"}</button>}</div>}
    {(event.status === "posted" || event.status === "compensated") && <div className="funding-action-choice"><p>Verify that the journal debit and credit still match the recorded exact evidence amount.</p><button className="button secondary" type="button" disabled={!online || busy} onClick={onReconcile}>Reconcile reference</button></div>}
    {event.status === "posted" && !event.compensation_of_event_id && !event.compensation_event_id && canWrite && <details className="funding-compensation-disclosure" open={compensating} onToggle={(toggle) => setCompensating(toggle.currentTarget.open)}><summary>Request an additive compensation</summary><p>The original event and journal remain immutable. Compensation requires its own review and balanced reversal.</p><form onSubmit={submitCompensation}><label>Reason code<select value={reasonCode} onChange={(change) => setReasonCode(change.target.value)}><option value="external_evidence_reversed">External evidence reversed</option><option value="duplicate_external_evidence">Duplicate external evidence</option><option value="operator_correction">Operator correction</option></select></label><label>Operator note<textarea required rows={3} maxLength={500} value={operatorNote} onChange={(change) => setOperatorNote(change.target.value)} placeholder="Describe the verified reversal evidence" /></label><button className="button danger guarded-control" type="submit" disabled={!online || busy || !operatorNote.trim()}>{busy ? "Recording…" : "Record compensation request"}</button></form></details>}
    {event.status === "rejected" && <div className="funding-terminal-note"><WarningCircle weight="fill" aria-hidden="true" /><p>This record is final and produced no journal posting. Record a new request only for a genuinely new external reference.</p></div>}
  </section>;
}
