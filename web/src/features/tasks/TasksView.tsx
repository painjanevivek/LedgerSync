"use client";
import Link from "next/link";
import type { ReconciliationRun, TransferSummary } from "@/features/accounts/types";
import { reconciliationPresentation, transferStatusPresentation } from "@/features/console/presentation";
import { approvalDetailHref, type ApprovalItem } from "@/lib/api/approvals";
import { ActionAvailability } from "@/ui/controls/ActionAvailability";
import { PageHeader } from "@/ui/display/PageHeader";
import { StatePanel } from "@/ui/display/StatePanel";
import { Money } from "@/ui/display/Money";
import { EmptyState } from "@/ui/presentation/EmptyState";
import { RecordIdentity } from "@/ui/presentation/RecordIdentity";
import { RelativeTime } from "@/ui/presentation/RelativeTime";
import { TaskCard } from "@/ui/presentation/TaskCard";
import { TechnicalDetails } from "@/ui/presentation/TechnicalDetails";
import { orderTasks, type TaskCoverage, type WorkspaceTask } from "./taskPresentation";

export function TasksView({ approvals, transfers, reconciliation, loading, verified, partial, online, errors, onRefresh, supplemental = [], coverage = [], retainedTransfer = false, storageUnavailable = false }: Readonly<{
  approvals: ApprovalItem[]; transfers: TransferSummary[]; reconciliation: ReconciliationRun | null;
  loading: boolean; verified: boolean; partial: boolean; online: boolean; errors: string[]; onRefresh: () => void;
  supplemental?: WorkspaceTask[]; coverage?: TaskCoverage[]; retainedTransfer?: boolean; storageUnavailable?: boolean;
}>) {
  const tasks: WorkspaceTask[] = approvals.filter(item => item.actionable_by_me || item.self_approval_blocked || !item.evidence_complete || item.step_up_status === "required").map(item => ({
    id: `${item.domain}:${item.record_id}`, title: item.domain === "funding" ? "Review money being added" : "Review a record correction",
    explanation: item.self_approval_blocked ? "A different authorized operator must review this request." : !item.evidence_complete ? "Supporting information must be completed before a decision can be made." : item.step_up_status === "required" ? "Confirm your identity before making this decision." : "Check the amount and supporting information, then record your decision.",
    tone: "warning", priority: 2, actionable: item.actionable_by_me, occurredAt: item.requested_at, amountMinor: item.amount_minor, currency: item.currency, reference: item.record_id, group: "attention", action: { label: item.actionable_by_me ? "Review now" : "Open request", href: approvalDetailHref(item, "/tasks") },
  }));
  for (const transfer of transfers.filter(item => item.financial_status !== "posted")) {
    const status = transferStatusPresentation(transfer.financial_status);
    tasks.push({ id: `transfer:${transfer.transfer_id}`, title: status.title, explanation: status.explanation, tone: status.tone, priority: transfer.financial_status === "pending" ? 0 : 4, actionable: transfer.financial_status === "pending", occurredAt: transfer.created_at, amountMinor: transfer.amount_minor, currency: transfer.currency, reference: transfer.transfer_id, group: transfer.financial_status === "pending" ? "attention" : "history", action: { label: transfer.financial_status === "pending" ? "Check status" : "View transfer", href: `/transfers/${transfer.transfer_id}` } });
  }
  if (reconciliation) {
    const status = reconciliationPresentation(reconciliation.status, reconciliation.mismatch_count);
    if (status.attention) tasks.push({ id: `balance:${reconciliation.run_id}`, title: status.title, explanation: status.explanation, tone: status.tone, priority: 1, actionable: true, occurredAt: reconciliation.started_at, reference: reconciliation.run_id, group: "attention", action: { label: "Review balance check", href: `/reconciliation/${reconciliation.run_id}` } });
  }
  if (retainedTransfer) tasks.push({ id: "local:transfer", title: "An earlier transfer is not confirmed", explanation: "This browser retains an original request, not a server-confirmed outcome. Do not create another transfer. Resolve that request first.", tone: "unknown", priority: 0, actionable: true, group: "attention", action: { label: "Review original request", href: "/transfers/new" } });
  const ordered = orderTasks([...tasks, ...supplemental]);
  const attention = ordered.filter(item => item.group === "attention");
  const incomplete = partial || coverage.some(source => source.state === "partial");
  const unavailable = errors.length > 0 || coverage.some(source => source.state === "unavailable");
  const checking = loading || !verified || coverage.some(source => source.state === "loading");
  const complete = !checking && online && !unavailable && !incomplete && !storageUnavailable;
  const title = attention.length ? `${attention.length} ${attention.length === 1 ? "item needs" : "items need"} attention` : !online || unavailable || storageUnavailable ? "Tasks need to be checked" : checking ? "Checking your tasks" : incomplete ? "More tasks may need attention" : "No attention items in the checked records";
  function renderTask(task: WorkspaceTask) {
    return <TaskCard key={task.id} title={task.title} explanation={task.explanation} tone={task.tone} action={task.action} context={<>{task.amountMinor && task.currency && <Money currency={task.currency} minorUnits={task.amountMinor} />}{task.occurredAt && <> · <RelativeTime value={task.occurredAt} /></>}</>} evidence={task.reference ? <TechnicalDetails><RecordIdentity value={task.reference} /></TechnicalDetails> : undefined} />;
  }
  return <>
    <PageHeader eyebrow="Tasks" title={title} description="Start with unresolved money movements, then work through reviews."><ActionAvailability availability={!online ? { state: "offline", reason: "Reconnect to check current tasks." } : loading ? { state: "busy", reason: "Wait for this refresh to finish." } : { state: "available" }}><button className="button secondary" type="button" onClick={onRefresh}>{loading ? "Refreshing…" : "Refresh"}</button></ActionAvailability></PageHeader>
    {!online && <StatePanel kind="unknown" title="These tasks may be out of date" message="Reconnect to check current work. Previously loaded records remain visible." />}
    {storageUnavailable && <StatePanel kind="unknown" announce="assertive" title="Saved browser requests could not be checked" message="Browser storage is unavailable. We cannot tell whether an earlier request is saved here. Restore access to browser storage before starting a new transfer; no saved request has been overwritten." action={<Link className="button secondary" href="/transfers/new">Check transfer recovery</Link>} />}
    {unavailable && <StatePanel kind="error" title="Some tasks could not be refreshed" message="Previously loaded work remains visible. This is not an all-clear. Retry when the connection is stable." />}
    {incomplete && <StatePanel title="This task list is incomplete" message="More records are available. Open the source lists below to review their remaining pages before treating the workspace as clear." />}
    {attention.length === 0 && complete && <EmptyState title="No urgent items in the checked records" message="This covers the sources and pages listed below—not work outside your access." action={<Link className="button secondary" href="/">Return home</Link>} />}
    <div className="task-list">{attention.map(renderTask)}</div>
    {(["setup", "history"] as const).map(group => ordered.some(task => task.group === group) && <section key={group}><h2>{group === "setup" ? "Setup to complete" : "Past unsuccessful requests"}</h2><p>{group === "history" ? "These are definitive past outcomes, not unconfirmed money movements." : "These prerequisites are separate from current financial incidents."}</p><div className="task-list">{ordered.filter(task => task.group === group).map(renderTask)}</div></section>)}
    <TechnicalDetails summary="What this task list covers" attention={incomplete || unavailable}>
      <ul className="task-source-coverage">{coverage.filter(source => source.state !== "not-authorized").map(source => <li key={source.id}><Link href={source.href}>{source.label}</Link><span>{({ loading: "Checking", verified: "Loaded page checked", partial: "More pages available", unavailable: "Could not be checked", "not-authorized": "Outside your access" })[source.state]}</span></li>)}</ul>
      <p>Only authorized sources are checked. Locally retained requests may overlap server records until their original result is resolved.</p>
      {errors.length > 0 && <TechnicalDetails summary="View technical details">{errors.map((error, index) => <p key={index}>{error}</p>)}</TechnicalDetails>}
    </TechnicalDetails>
  </>;
}
