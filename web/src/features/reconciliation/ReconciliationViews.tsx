"use client";

import { CheckCircle, HourglassMedium, ShieldWarning } from "@phosphor-icons/react";
import Link from "next/link";

import type { ReconciliationRun } from "@/features/accounts/types";
import { CopyControl } from "@/ui/controls/CopyControl.client";
import { FocusedRetry } from "@/ui/controls/FocusedRetry.client";
import { DataTableRegion } from "@/ui/display/DataTableRegion";
import { EvidenceFreshness } from "@/ui/display/Evidence";
import { PageHeader } from "@/ui/display/PageHeader";
import { RecordLink } from "@/ui/display/RecordLink";
import { StatePanel } from "@/ui/display/StatePanel";
import { StatusBadge } from "@/ui/display/StatusBadge";
import { Timestamp } from "@/ui/display/Timestamp";
import { EvidenceExportControl } from "@/features/exports/EvidenceExportControl";
import { ReconciliationCommand } from "@/features/reconciliation/ReconciliationCommand";
import { RelatedEvidenceRail } from "@/features/investigation/RelatedEvidenceRail";
import { reconciliationURL, type ReconciliationFilters } from "@/lib/page-query/reconciliation";
import { DisclosureSection } from "@/ui/disclosure/DisclosureSection";

function runTone(status: ReconciliationRun["status"]) {
  return status === "matched" ? "success" as const : status === "mismatch" || status === "failed" ? "danger" as const : "warning" as const;
}

function runLabel(status: ReconciliationRun["status"]) {
  return status === "matched" ? "Passed" : status === "mismatch" ? "Mismatch detected" : status === "failed" ? "Failed" : "Running";
}

function RunResult({ run }: Readonly<{ run: ReconciliationRun }>) {
  const running = run.status === "running";
  const passed = run.status === "matched" && run.mismatch_count === "0" && Boolean(run.ledger_watermark) && Boolean(run.completed_at);
  return <>
    <section className={`evidence-hero ${passed ? "" : "unavailable"}`}>
      {passed ? <CheckCircle weight="fill" aria-hidden="true" /> : running ? <HourglassMedium weight="fill" aria-hidden="true" /> : <ShieldWarning weight="fill" aria-hidden="true" />}
      <div><p className="eyebrow">{running ? "Authoritative run in progress" : "Authoritative completed run"}</p><h2>{runLabel(run.status)}</h2><p>{running ? "No pass or mismatch result is inferred while this run is active." : `${run.checked_account_count} accounts · ${run.posting_count} postings · ${run.mismatch_count} mismatches`}</p></div>
      <StatusBadge tone={runTone(run.status)}>{runLabel(run.status)}</StatusBadge>
    </section>
    <section className="identity-strip"><div><span>Run ID</span><CopyControl value={run.run_id} /></div><div><span>Scope</span><strong>{run.scope || "Recorded tenant scope"}</strong></div><div><span>Ledger watermark</span><strong>{run.ledger_watermark || "Pending authoritative capture"}</strong></div><div><span>{running ? "Started" : "Completed"}</span><strong><Timestamp value={running ? run.started_at : run.completed_at} /></strong></div></section>
    {run.mismatch_count !== "0" && <StatePanel kind="error" title={`${run.mismatch_count} mismatch${run.mismatch_count === "1" ? "" : "es"} require investigation`} message="This financial control did not pass. A mismatch cannot be marked resolved in the console; only a new authoritative run can establish later proof." />}
    {run.mismatches && run.mismatches.length > 0 && <section className="ledger-section reconciliation-mismatches" aria-labelledby="reconciliation-mismatches-heading">
      <div className="section-heading"><div><p className="eyebrow">Stop-ship evidence</p><h3 id="reconciliation-mismatches-heading">Affected records</h3><p>Open only tenant-authorized account details. Exact mismatch values remain tied to this immutable run.</p></div></div>
      <DataTableRegion label="Reconciliation mismatch evidence"><table className="data-table"><thead><tr><th>Mismatch ID</th><th>Classification</th><th>Account</th><th>Expected / observed</th><th>Balance version</th></tr></thead><tbody>{run.mismatches.map((mismatch) => <tr key={mismatch.mismatch_id}><td><CopyControl value={mismatch.mismatch_id} /></td><td>{mismatch.classification}</td><td>{mismatch.account_id ? <RecordLink href={`/accounts/${encodeURIComponent(mismatch.account_id)}`} label="Open affected account" id={`mismatch-account-${mismatch.mismatch_id}`} /> : "No account record supplied"}</td><td className="number-cell">{mismatch.currency || "INR"} {mismatch.expected_minor ?? "Unavailable"} / {mismatch.observed_minor ?? mismatch.observed_available_minor ?? "Unavailable"}</td><td><code>{mismatch.balance_version ?? "Unavailable"}</code></td></tr>)}</tbody></table></DataTableRegion>
    </section>}
  </>;
}

type ReconciliationViewProps = Readonly<{
  runs: ReconciliationRun[]; detail: ReconciliationRun | null; detailRequested: boolean; error: string | null; loading: boolean; verifiedAt?: string; nextCursor?: string; tenantId: string; csrfToken: string; online: boolean; canWrite: boolean; canExport: boolean; returnTo?: string; filters: ReconciliationFilters; onObserved: (run: ReconciliationRun) => void; onRefresh: () => Promise<void>;
}>;

export function ReconciliationView({ runs, detail, detailRequested, error, loading, verifiedAt, nextCursor, tenantId, csrfToken, online, canWrite, canExport, returnTo, filters, onObserved, onRefresh }: ReconciliationViewProps) {
  if (detail) return <>
    <PageHeader eyebrow="Ledger / Reconciliation" title="Reconciliation details" description="See whether account balances match the ledger for this check."><EvidenceExportControl label="Export run result" subject="reconciliation run" endpoint={`/api/exports/reconciliation.csv?runId=${encodeURIComponent(detail.run_id)}&limit=10000`} scope={`One immutable run · ${detail.run_id}`} filters={[{ label: "Run ID", value: detail.run_id }]} columns="Includes run, mismatch, account, and correlation identifiers when present" online={online} canExport={canExport} /></PageHeader>
    <RunResult run={detail} />
    <DisclosureSection id="reconciliation-technical-evidence" title="Technical run evidence" summary="Correlation, application version, and exact timestamps." lazy><section className="surface detail-document"><dl className="evidence-list"><div><dt>Correlation ID</dt><dd><CopyControl value={detail.correlation_id} /></dd></div><div><dt>Application version</dt><dd>{detail.application_version || "Unavailable"}</dd></div><div><dt>Started</dt><dd><Timestamp value={detail.started_at} /></dd></div><div><dt>Completed</dt><dd><Timestamp value={detail.completed_at} /></dd></div></dl></section></DisclosureSection>
    <DisclosureSection id="reconciliation-related-evidence" title="Related evidence" summary="Open authorized records linked to this run." lazy><RelatedEvidenceRail sourceType="reconciliation_run" sourceId={detail.run_id} /></DisclosureSection>
    <Link className="text-link back-link" href={returnTo ?? "/reconciliation"}>← Back to previous view</Link>
  </>;

  if (detailRequested) return <>
    <PageHeader eyebrow="Control record / Immutable run" title="Reconciliation detail" description="Loading the requested tenant-authorized control record." />
    {error ? <StatePanel kind="error" title="Reconciliation result unavailable" message={error} action={<FocusedRetry label="Retry this run only" onRetry={() => void onRefresh()} disabled={!online} busy={loading} />} /> : <StatePanel title="Loading reconciliation result" message="No passing or mismatch result is inferred while the authoritative run is loading." />}
  </>;

  const latest = runs[0];
  const listReturnTo = reconciliationURL(filters);
  const nextHref = nextCursor ? reconciliationURL({ cursor: nextCursor }) : undefined;
  return <>
    <PageHeader eyebrow="Ledger / Reconciliation" title="Reconciliation" description="Check that account balances match the ledger records."><button className="button secondary" type="button" disabled={loading || !online} onClick={() => void onRefresh()}>{loading ? "Refreshing results…" : "Refresh results"}</button></PageHeader>
    {error && <StatePanel kind="error" title="Reconciliation results unavailable" message={error} action={<FocusedRetry label="Retry reconciliation history only" onRetry={() => void onRefresh()} disabled={!online} busy={loading} />} />}
    {verifiedAt && latest && <EvidenceFreshness state={error || !online ? "historical" : loading ? "refreshing" : "current"} verifiedAt={verifiedAt} label="Reconciliation results" reason={error ?? (!online ? "Reconnect before treating this run as current." : undefined)} />}
    {loading && !latest ? <StatePanel title="Loading reconciliation results" message="No result is inferred until an authoritative run has been returned." /> : !error && !latest ? <StatePanel kind="unknown" title="No reconciliation results" message="The verified history is empty. LedgerSync cannot claim a passing result until an authoritative completed run exists." /> : latest && <RunResult run={latest} />}
    <ReconciliationCommand tenantId={tenantId} csrfToken={csrfToken} online={online} canWrite={canWrite} evidenceReady={!error && !loading} latestRun={latest ?? null} onObserved={onObserved} onRefreshHistory={onRefresh} />
    {runs.length > 0 && <DisclosureSection id="reconciliation-history" title="Previous reconciliation runs" summary={`${runs.length} immutable run${runs.length === 1 ? "" : "s"} on this page`} lazy><section className="ledger-section">
      <div className="section-heading"><div><p className="eyebrow">Newest completed evidence first</p><h2>Reconciliation runs</h2><p>{runs.length} run{runs.length === 1 ? "" : "s"} on this page. A total is not calculated or implied. Starting a new run never replaces prior results.</p></div><EvidenceExportControl label="Export reconciliation results" subject="reconciliation history" endpoint="/api/exports/reconciliation.csv?limit=10000" scope="All authorized reconciliation runs and mismatch details; the cursor only selects the visible page" filters={[]} columns="Includes run, mismatch, account, and correlation identifiers when present" online={online} canExport={canExport} /></div>
      <DataTableRegion label="Reconciliation run history"><table className="data-table"><thead><tr><th>Run ID</th><th>Result</th><th>Scope</th><th>Accounts / postings</th><th>Mismatches</th><th>Completed UTC</th><th>Action</th></tr></thead><tbody>{runs.map((run) => <tr key={run.run_id}><td><CopyControl value={run.run_id} /></td><td><StatusBadge tone={runTone(run.status)}>{runLabel(run.status)}</StatusBadge></td><td>{run.scope}</td><td>{run.checked_account_count} / {run.posting_count}</td><td className="number-cell">{run.mismatch_count}</td><td>{run.status === "running" ? "In progress" : <Timestamp value={run.completed_at} />}</td><td><RecordLink href={`/reconciliation/${run.run_id}?return_to=${encodeURIComponent(listReturnTo)}`} label="Open result" /></td></tr>)}</tbody></table></DataTableRegion>
      <div className="pagination"><span>{nextHref ? "More reconciliation runs are available" : "End of reconciliation history"}</span>{nextHref ? <Link className="button secondary" href={nextHref}>Next page</Link> : <button className="button secondary" type="button" disabled>Next page</button>}</div>
    </section></DisclosureSection>}
  </>;
}
