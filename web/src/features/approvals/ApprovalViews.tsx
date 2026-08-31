"use client";

import Link from "next/link";
import { FormEvent, useState } from "react";

import type { ConsoleCapabilities } from "@/features/console/capabilities";
import {
  CopyControl,
  DataTableRegion,
  EvidenceFreshness,
  FormField,
  PageHeader,
  StatePanel,
  StatusBadge,
} from "@/features/console/components";
import { utcDateTime } from "@/features/console/format";
import type {
  ApprovalFilters,
  ApprovalItem,
} from "@/lib/api/approvals";
import { approvalDetailHref } from "@/lib/api/approvals";
import { formatMinorUnits } from "@/lib/money";

const statusOptions = [
  ["funding:requested", "Funding — requested"],
  ["funding:approved", "Funding — approved"],
  ["funding:posted", "Funding — posted"],
  ["funding:rejected", "Funding — rejected"],
  ["funding:compensated", "Funding — compensated"],
  ["correction:requested", "Correction — requested"],
  ["correction:approved", "Correction — approved"],
  ["correction:rejected", "Correction — rejected"],
  ["correction:cancelled", "Correction — cancelled"],
  ["correction:expired", "Correction — expired"],
  ["correction:posted", "Correction — posted"],
] as const;

function statusTone(status: string) {
  if (status === "posted") return "success" as const;
  if (status === "rejected" || status === "cancelled" || status === "expired") return "danger" as const;
  return "warning" as const;
}

function ageLabel(ageSeconds: string) {
  const seconds = /^\d+$/.test(ageSeconds) ? BigInt(ageSeconds) : 0n;
  const hours = seconds / 3_600n;
  if (hours < 24n) return `${hours}h old`;
  return `${hours / 24n}d old`;
}

function actionLabel(item: ApprovalItem) {
  switch (item.safe_next_action) {
    case "wait_for_independent_approver": return "Open blocked record";
    case "complete_evidence": return "Inspect missing evidence";
    case "reauthenticate": return "Reauthenticate to review";
    case "review_decision": return "Review decision";
    default: return "Open record";
  }
}

export function ApprovalFiltersForm({
  filters,
  capabilities,
  busy,
  onApply,
  onClear,
}: Readonly<{
  filters: ApprovalFilters;
  capabilities: ConsoleCapabilities;
  busy: boolean;
  onApply: (filters: ApprovalFilters) => void;
  onClear: () => void;
}>) {
  const [draft, setDraft] = useState(filters);

  function submit(event: FormEvent) {
    event.preventDefault();
    onApply({ ...draft, requester: draft.requester.trim(), cursor: undefined });
  }

  return (
    <form className="approval-filters surface" onSubmit={submit}>
      <FormField label="Approval domain" requirement="optional">
        <select value={draft.domain} onChange={(event) => setDraft({ ...draft, domain: event.target.value as ApprovalFilters["domain"], status: "" })}>
          <option value="">All authorized domains</option>
          {capabilities.fundingApprove ? <option value="funding">Funding</option> : null}
          {capabilities.correctionsApprove ? <option value="correction">Corrections</option> : null}
        </select>
      </FormField>
      <FormField label="Exact domain status" requirement="optional" hint="Statuses remain domain-qualified; they are never collapsed into a generic pending state.">
        <select value={draft.status} onChange={(event) => setDraft({ ...draft, status: event.target.value })}>
          <option value="">All exact statuses</option>
          {statusOptions.filter(([value]) => {
            const funding = value.startsWith("funding:");
            return (!draft.domain || value.startsWith(`${draft.domain}:`)) && (funding ? capabilities.fundingApprove : capabilities.correctionsApprove);
          }).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </select>
      </FormField>
      <FormField label="Requester subject" requirement="optional" hint="Exact server-owned subject reference; no fuzzy identity lookup is performed.">
        <input maxLength={255} value={draft.requester} onChange={(event) => setDraft({ ...draft, requester: event.target.value })} />
      </FormField>
      <FormField label="Age" requirement="optional">
        <select value={draft.age} onChange={(event) => setDraft({ ...draft, age: event.target.value as ApprovalFilters["age"] })}>
          <option value="">Any age</option>
          <option value="under_24h">Under 24 hours</option>
          <option value="over_24h">24 hours or older</option>
          <option value="over_7d">7 days or older</option>
          <option value="over_30d">30 days or older</option>
        </select>
      </FormField>
      <FormField label="Requested from (UTC)" requirement="optional"><input type="date" value={draft.requestedAfter} onChange={(event) => setDraft({ ...draft, requestedAfter: event.target.value })} /></FormField>
      <FormField label="Requested through (UTC)" requirement="optional"><input type="date" value={draft.requestedBefore} onChange={(event) => setDraft({ ...draft, requestedBefore: event.target.value })} /></FormField>
      <label className="approval-actionable-filter"><input type="checkbox" checked={draft.actionableByMe} onChange={(event) => setDraft({ ...draft, actionableByMe: event.target.checked })} /><span>Actionable by me only</span></label>
      <div className="action-row">
        <button className="button primary" type="submit" disabled={busy}>Apply filters</button>
        <button className="button secondary" type="button" disabled={busy} onClick={onClear}>Clear all</button>
      </div>
    </form>
  );
}

export function ApprovalList({
  items,
  pageCount,
  nextHref,
  returnTo,
  loading,
}: Readonly<{
  items: ApprovalItem[];
  pageCount: number;
  nextHref?: string;
  returnTo: string;
  loading: boolean;
}>) {
  if (loading && items.length === 0) return <StatePanel title="Loading approval evidence" message="Requesting the oldest bounded authorized page. No approval state is inferred while it loads." />;
  if (items.length === 0) return <StatePanel title="No approvals match these filters" message="The authorized page is empty. Clear or change filters; this does not imply other domains or unavailable evidence are empty." />;
  return (
    <section className="ledger-section" aria-labelledby="approval-queue-heading" aria-busy={loading}>
      <div className="section-heading"><div><p className="eyebrow">Oldest actionable evidence first</p><h2 id="approval-queue-heading">Independent review queue</h2><p>{pageCount} record{pageCount === 1 ? "" : "s"} on this page. A total is not calculated or implied.</p></div></div>
      <DataTableRegion label="Approval queue records">
        <table className="data-table approval-table">
          <thead><tr><th scope="col">Domain and record</th><th scope="col">Requester and age</th><th scope="col">Exact value</th><th scope="col">Status</th><th scope="col">Decision evidence</th><th scope="col">Safe next action</th></tr></thead>
          <tbody>{items.map((item) => {
            const detailHref = approvalDetailHref(item, returnTo);
            const actionHref = item.safe_next_action === "reauthenticate" ? `/api/auth/sign-in?prompt=login&return_to=${encodeURIComponent(detailHref)}` : detailHref;
            return <tr key={`${item.domain}:${item.record_id}`}>
              <td><strong>{item.domain === "funding" ? "Funding" : "Correction"}</strong><CopyControl value={item.record_id} label={`Copy ${item.domain} record ID`} />{item.related_account_id ? <span>Account <code>{item.related_account_id}</code></span> : null}{item.related_transfer_id ? <span>Transfer <code>{item.related_transfer_id}</code></span> : null}</td>
              <td><code>{item.requester_subject_id}</code><time dateTime={item.requested_at}>{utcDateTime(item.requested_at)}</time><span>{ageLabel(item.age_seconds)}</span></td>
              <td><strong>{formatMinorUnits(item.currency, item.amount_minor)}</strong><span>{item.currency} minor units: {item.amount_minor}</span></td>
              <td><StatusBadge tone={statusTone(item.status)}>{item.status}</StatusBadge><span>{item.required_scope}</span>{item.approval_expires_at ? <span>Expires {utcDateTime(item.approval_expires_at)}</span> : null}</td>
              <td>{item.evidence_complete ? <StatusBadge tone="success">evidence complete</StatusBadge> : <StatusBadge tone="danger">evidence incomplete</StatusBadge>}{item.self_approval_blocked ? <span className="approval-stop">Self-approval blocked</span> : <span>Independent actor allowed</span>}<span>Step-up: {item.step_up_status.replaceAll("_", " ")}</span></td>
              <td><Link className={item.safe_next_action === "review_decision" ? "button primary" : "button secondary"} href={actionHref}>{actionLabel(item)}</Link></td>
            </tr>;
          })}</tbody>
        </table>
      </DataTableRegion>
      <div className="pagination"><span>{nextHref ? "More matching approval evidence is available" : "End of this filtered approval queue"}</span>{nextHref ? <Link className="button secondary" href={nextHref}>Next page</Link> : <button className="button secondary" type="button" disabled>Next page</button>}</div>
    </section>
  );
}

export function ApprovalHeader({ verifiedAt, loading, error }: Readonly<{ verifiedAt?: string; loading: boolean; error: string | null }>) {
  return <><PageHeader eyebrow="Work / Independent decisions" title="Approvals" description="Review the oldest authorized funding and correction decisions without mixing approval authority with permanent posting." />{verifiedAt ? <EvidenceFreshness state={error ? "historical" : loading ? "refreshing" : "current"} verifiedAt={verifiedAt} label="Approval queue" reason={error ?? undefined} /> : null}</>;
}
