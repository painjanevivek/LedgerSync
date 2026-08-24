"use client";

import { CheckCircle, Clock, LinkSimple, WarningCircle } from "@phosphor-icons/react";
import Link from "next/link";

import { CopyControl, StatePanel, StatusBadge } from "@/features/console/components";
import { utcDateTime } from "@/features/console/format";
import type { ExplainabilityEvidence, ExplainabilityStage, TransferExplainability } from "@/lib/api/orientation";
import { formatMinorUnits } from "@/lib/money";

const stageCopy: Record<ExplainabilityStage["kind"], { title: string; description: string }> = {
  request: { title: "Request and idempotency outcome", description: "The retained command outcome for the original intent, when stored." },
  transfer: { title: "Posted transfer", description: "The authoritative transfer record; rejection never implies postings." },
  journal_postings: { title: "Journal and debit / credit postings", description: "Immutable double-entry records linked to the transfer." },
  balance_versions: { title: "Account balance versions", description: "Stored versions associated with the committed financial result." },
  outbox: { title: "Outbox events", description: "Committed delivery work, separate from the financial posting." },
  delivery: { title: "Downstream delivery", description: "Published, retry, or dead delivery-attempt evidence." },
  reconciliation: { title: "Reconciliation coverage", description: "A stored run that explicitly covers the ledger watermark, when provable." },
};
const missingCopy: Record<string, string> = {
  no_retained_idempotency_outcome: "No retained idempotency outcome is linked to this transfer.", no_journal: "No journal is stored for this transfer.", no_postings: "No debit or credit postings are stored.", no_balance_version_evidence: "No stored account balance-version evidence is linked.", no_outbox_events: "No outbox event is linked.", no_delivery_attempts: "No downstream delivery attempt is stored.", coverage_not_provable: "No reconciliation run proves coverage of this ledger watermark.", dependency_unavailable: "The dependency needed for this stage is unavailable.", evidence_truncated: "More evidence exists than this bounded read model returned.",
};

function relatedHref(item: ExplainabilityEvidence, backTo: string) {
  const encodedBack = encodeURIComponent(backTo);
  if (item.account_id) return `/accounts/${encodeURIComponent(item.account_id)}?return_to=${encodedBack}`;
  if (item.evidence_type === "outbox_event" && item.evidence_id) return `/events/${encodeURIComponent(item.evidence_id)}?return_to=${encodedBack}`;
  if (item.evidence_type === "reconciliation_run" && item.evidence_id) return `/reconciliation/${encodeURIComponent(item.evidence_id)}?return_to=${encodedBack}`;
  return null;
}
function stageTone(stage: ExplainabilityStage) { return stage.state === "available" && !stage.truncated ? "success" as const : "warning" as const; }
function stageTimes(stage: ExplainabilityStage) { return stage.evidence.map((item) => Date.parse(item.occurred_at)).filter(Number.isFinite); }

export function TransferEvidenceTimeline({ evidence, loading, error, online, canRead, transferId, backTo, onRefresh }: Readonly<{ evidence: TransferExplainability | null; loading: boolean; error: string | null; online: boolean; canRead: boolean; transferId: string; backTo: string; onRefresh: () => void }>) {
  if (!canRead) return <StatePanel kind="denied" title="Stored evidence timeline not authorized" message="This linked view requires explainability, transfer, event, and reconciliation read scopes. The financial transfer detail above remains unchanged." />;
  if (!online) return <StatePanel kind="offline" title="Stored evidence timeline unavailable offline" message="LedgerSync will not present retained browser data as current linked evidence." />;
  if (error) return <StatePanel kind="error" title="Stored evidence timeline unavailable" message={error} action={<button className="button secondary" type="button" onClick={onRefresh}>Retry timeline</button>} />;
  if (loading || !evidence) return <StatePanel title="Loading stored evidence timeline" message="Seven independently stored stages are being verified without inferring missing links." />;

  let latest = Number.NEGATIVE_INFINITY; let outOfOrder = false;
  for (const stage of evidence.stages) { const times = stageTimes(stage); if (times.some((time) => time < latest)) outOfOrder = true; if (times.length) latest = Math.max(latest, ...times); }
  return <section className="transfer-evidence-chain" aria-labelledby="stored-evidence-title">
    <header><div><p className="eyebrow">Unified explainability / Read model</p><h2 id="stored-evidence-title">Stored evidence chain</h2><p>Semantic order is fixed. Missing, unavailable, and truncated stages remain visible and are never promoted into financial, delivery, or reconciliation truth.</p></div><button className="button secondary" type="button" onClick={onRefresh}>Refresh linked evidence</button></header>
    {outOfOrder && <StatePanel kind="unknown" title="Stored timestamps are out of sequence" message="The records remain in semantic evidence order. LedgerSync has not reordered or reconciled their timestamps." />}
    <ol>
      {evidence.stages.map((stage) => <li key={stage.sequence} className={`evidence-stage ${stage.state}`}>
        <div className="evidence-stage-marker"><span>{stage.sequence}</span>{stage.state === "available" && !stage.truncated ? <CheckCircle weight="fill" aria-hidden="true" /> : stage.state === "unavailable" ? <WarningCircle weight="fill" aria-hidden="true" /> : <Clock weight="fill" aria-hidden="true" />}</div>
        <article>
          <header><div><h3>{stageCopy[stage.kind].title}</h3><p>{stageCopy[stage.kind].description}</p></div><StatusBadge tone={stageTone(stage)}>{stage.state === "available" ? stage.truncated ? "Available · bounded" : "Available" : stage.state === "missing" ? "Missing" : "Unavailable"}</StatusBadge></header>
          {stage.evidence.length > 0 ? <ul className="stage-evidence-list">{stage.evidence.map((item, index) => {
            const href = relatedHref(item, backTo);
            return <li key={`${item.evidence_type}-${item.evidence_id ?? index}`}><div><strong>{item.evidence_type.replaceAll("_", " ")}</strong><time>{utcDateTime(item.occurred_at)}</time></div>{item.status && <StatusBadge tone={item.status === "posted" || item.status === "published" || item.status === "delivered" || item.status === "matched" || item.status === "completed" ? "success" : item.status === "rejected" || item.status === "dead" || item.status === "failed" || item.status === "mismatch" ? "danger" : "warning"}>{item.status}</StatusBadge>}{item.amount_minor !== undefined && item.currency && <strong className="number-cell">{formatMinorUnits(item.currency, item.amount_minor)}</strong>}{item.balance_version !== undefined && <span className="mono">version {item.balance_version}</span>}{item.attempt_number !== undefined && <span className="mono">attempt {item.attempt_number}</span>}{href ? <Link className="record-link" href={href}><LinkSimple aria-hidden="true"/>Open related evidence</Link> : item.evidence_id ? <CopyControl value={item.evidence_id} /> : null}</li>;
          })}</ul> : <p className="stage-gap"><WarningCircle weight="fill" aria-hidden="true"/><span>{stage.reason_code ? missingCopy[stage.reason_code] : "No stored evidence was returned for this stage."}</span></p>}
          {stage.truncated && <p className="stage-gap"><WarningCircle weight="fill" aria-hidden="true"/><span>This bounded stage is incomplete. Open its related evidence view for the authorized detail.</span></p>}
        </article>
      </li>)}
    </ol>
    <footer><span>Generated {utcDateTime(evidence.generated_at)}</span><span>Transfer {transferId}</span></footer>
  </section>;
}
