"use client";

import { ArrowClockwise, CheckCircle, Database, HardDrives, Lightning, WarningCircle } from "@phosphor-icons/react";

import { CopyControl } from "@/ui/controls/CopyControl.client";
import { EvidenceFreshness } from "@/ui/display/Evidence";
import { PageHeader } from "@/ui/display/PageHeader";
import { RecordLink } from "@/ui/display/RecordLink";
import { StatePanel } from "@/ui/display/StatePanel";
import { StatusBadge } from "@/ui/display/StatusBadge";
import type { DependencyState, LocalDiagnostics, OperationalState } from "@/lib/api/operations";

function stateTone(state: OperationalState | DependencyState) {
  return state === "ready" || state === "reachable" ? "success" as const : state === "degraded" ? "warning" as const : "danger" as const;
}

function stateLabel(state: string) { return state.replaceAll("_", " ").replace(/^./, (value) => value.toUpperCase()); }

function utc(value?: string) {
  if (!value) return "Not available";
  const date = new Date(value);
  return Number.isNaN(date.valueOf()) ? "Not available" : `${date.toISOString().replace("T", " ").replace(".000Z", " UTC")}`;
}

export type LocalCapabilityGuidance = Readonly<{ id: string; capability: string; recovery: string; command?: string }>;

export function localCapabilityGuidance(evidence: LocalDiagnostics): LocalCapabilityGuidance[] {
  const guidance: LocalCapabilityGuidance[] = [];
  if (evidence.financial_authority.postgres.state === "unavailable") guidance.push({ id: "postgres", capability: "Authoritative balances, account history, transfers, and reconciliation are unavailable.", recovery: "Start the local runtime, then run the read-only status check. Do not retry a financial write until PostgreSQL is reachable.", command: "powershell -File .\\scripts\\start-local.ps1" });
  if (evidence.financial_authority.latest_reconciliation.state === "none") guidance.push({ id: "reconciliation-none", capability: "No completed reconciliation control exists; the console cannot claim a zero-mismatch result.", recovery: "Open Reconciliation and start one authorized run after current history and PostgreSQL readiness are verified." });
  if (evidence.financial_authority.latest_reconciliation.state === "unavailable") guidance.push({ id: "reconciliation-unavailable", capability: "The latest reconciliation result cannot be verified.", recovery: "Restore PostgreSQL/API readiness, then refresh only reconciliation evidence. Preserve the last verified run as historical.", command: "powershell -File .\\scripts\\status-local.ps1" });
  if (evidence.delivery_cache.outbox.state === "unavailable" || evidence.delivery_cache.outbox.worker_progress === "stalled" || evidence.delivery_cache.outbox.dead_count !== undefined && evidence.delivery_cache.outbox.dead_count !== "0") guidance.push({ id: "outbox", capability: "Downstream event delivery and delivery-status freshness are affected; committed PostgreSQL money is not reversed.", recovery: "Inspect timestamped worker logs and event evidence, then restart the runtime only if the worker remains unhealthy.", command: "powershell -File .\\scripts\\logs-local.ps1 -Service outbox-worker" });
  if (evidence.delivery_cache.redis.state === "unavailable") guidance.push({ id: "redis", capability: "Cache-accelerated reads are affected; PostgreSQL financial authority remains unchanged.", recovery: "Run the local status check, then restart the local runtime if Redis remains unavailable. Never rebuild money from Redis.", command: "powershell -File .\\scripts\\status-local.ps1" });
  return guidance;
}

export function LocalStatusView({ evidence, verifiedAt, loading, error, online, canRead, onRefresh }: Readonly<{ evidence: LocalDiagnostics | null; verifiedAt?: string; loading: boolean; error: string | null; online: boolean; canRead: boolean; onRefresh: () => void }>) {
  const guidance = evidence ? localCapabilityGuidance(evidence) : [];
  return <>
    <PageHeader eyebrow="Local tools / Status" title="Local status" description="See what is working and what needs attention before you take a recovery step.">
      <button className="button secondary guarded-control" type="button" disabled={!online || loading || !canRead} onClick={onRefresh}><ArrowClockwise aria-hidden="true" />{loading ? "Refreshing evidence…" : "Refresh evidence"}</button>
    </PageHeader>
    {!canRead && <StatePanel kind="denied" title="Local diagnostics not authorized" message="This session does not include local:read. No dependency evidence has been requested." />}
    {!online && <StatePanel kind="offline" title="Offline — evidence is not current" message={evidence ? "The last verified snapshot remains visible with its generation time. Reconnect before treating it as current." : "Reconnect to request a verified local status snapshot."} />}
    {error && <StatePanel kind="error" title="Local status unavailable" message={error} />}
    {verifiedAt && evidence && <EvidenceFreshness state={error || !online ? "historical" : loading ? "refreshing" : "current"} verifiedAt={verifiedAt} label="Diagnostic snapshot" reason={error ?? (!online ? "Reconnect before treating dependency state as current." : undefined)} />}
    {loading && !evidence && <StatePanel title="Loading truth domains" message="Requesting a bounded diagnostics snapshot. No dependency state is being inferred." />}
    {evidence && <section className="operations-status" aria-labelledby="operations-status-heading" aria-busy={loading}>
      <div className={`operations-verdict ${evidence.overall_state}`}>
        {evidence.overall_state === "ready" ? <CheckCircle weight="fill" aria-hidden="true" /> : <WarningCircle weight="fill" aria-hidden="true" />}
        <div><p className="eyebrow">Overall local state</p><h2 id="operations-status-heading">{stateLabel(evidence.overall_state)}</h2><p>{evidence.overall_state === "ready" ? "Financial authority and delivery evidence are reachable in this bounded snapshot." : evidence.overall_state === "degraded" ? "At least one supporting domain needs attention. Inspect each lane; do not infer a financial failure from delivery or cache state." : "The snapshot cannot prove all required local evidence. Financial results remain unknown unless separately verified."}</p></div>
        <StatusBadge tone={stateTone(evidence.overall_state)}>{stateLabel(evidence.overall_state)}</StatusBadge>
      </div>

      <div className="truth-ledger" role="region" aria-label="Ordered local truth domains">
        <article className="truth-lane financial-truth"><div className="truth-order">01</div><Database aria-hidden="true"/><div className="truth-copy"><p className="eyebrow">Financial authority</p><h3>PostgreSQL ledger</h3><p>Balances, postings, and reconciliation evidence are authoritative only here.</p></div><StatusBadge tone={stateTone(evidence.financial_authority.postgres.state)}>{stateLabel(evidence.financial_authority.postgres.state)}</StatusBadge><dl><div><dt>Schema version</dt><dd><code>{evidence.financial_authority.postgres.schema_version ?? "Not available"}</code></dd></div><div><dt>Latest reconciliation</dt><dd>{stateLabel(evidence.financial_authority.latest_reconciliation.state)}{evidence.financial_authority.latest_reconciliation.status ? ` · ${stateLabel(evidence.financial_authority.latest_reconciliation.status)}` : ""}</dd></div></dl>{evidence.financial_authority.latest_reconciliation.run_id && <RecordLink href={`/reconciliation/${encodeURIComponent(evidence.financial_authority.latest_reconciliation.run_id)}`} label="Open reconciliation evidence" />}</article>
        <article className="truth-lane delivery-truth"><div className="truth-order">02</div><Lightning aria-hidden="true"/><div className="truth-copy"><p className="eyebrow">Downstream delivery</p><h3>Transactional outbox</h3><p>Delivery can lag or fail after a financial command has already committed.</p></div><StatusBadge tone={stateTone(evidence.delivery_cache.outbox.state)}>{stateLabel(evidence.delivery_cache.outbox.state)}</StatusBadge><dl><div><dt>Pending</dt><dd>{evidence.delivery_cache.outbox.pending_count ?? "Not available"}</dd></div><div><dt>Dead</dt><dd>{evidence.delivery_cache.outbox.dead_count ?? "Not available"}</dd></div><div><dt>Worker progress</dt><dd>{stateLabel(evidence.delivery_cache.outbox.worker_progress)}</dd></div><div><dt>Oldest pending</dt><dd>{utc(evidence.delivery_cache.outbox.oldest_pending_at)}</dd></div></dl><RecordLink href="/events" label="Investigate delivery events" /></article>
        <article className="truth-lane cache-truth"><div className="truth-order">03</div><HardDrives aria-hidden="true"/><div className="truth-copy"><p className="eyebrow">Disposable acceleration</p><h3>Redis cache</h3><p>A cache outage may slow reads. It is never evidence that PostgreSQL money changed.</p></div><StatusBadge tone={stateTone(evidence.delivery_cache.redis.state)}>{stateLabel(evidence.delivery_cache.redis.state)}</StatusBadge><dl><div><dt>Role</dt><dd>Disposable cache</dd></div><div><dt>Fallback</dt><dd>Version-checked authoritative read</dd></div></dl></article>
      </div>

      <section className="ledger-section capability-guidance-section" aria-labelledby="capability-impact-heading"><div className="section-heading"><div><p className="eyebrow">Dependency-aware recovery</p><h2 id="capability-impact-heading">Affected capabilities and exact next actions</h2><p>Each action is scoped to the dependency evidence above. It does not assert or reverse a financial outcome.</p></div></div>{guidance.length === 0 ? <StatePanel title="No affected capability in this snapshot" message="All required domains are reachable and a completed reconciliation result is available. Continue to use record-level evidence for financial decisions." /> : <div className="capability-guidance-list">{guidance.map((item) => <article className="capability-guidance" key={item.id}><div><p><strong>Affected capability</strong><span>{item.capability}</span></p><p><strong>Recovery action</strong><span>{item.recovery}</span></p></div>{item.command && <CopyControl value={item.command} label={`Copy ${item.id} recovery command`} />}</article>)}</div>}</section>

      <section className="operations-environment" aria-labelledby="environment-evidence-heading"><div><p className="eyebrow">Snapshot provenance</p><h2 id="environment-evidence-heading">Environment evidence</h2></div><dl><div><dt>Environment</dt><dd>{evidence.application.environment}</dd></div><div><dt>Application version</dt><dd><code>{evidence.application.version}</code></dd></div><div><dt>Commit</dt><dd><CopyControl value={evidence.application.commit} label="Copy application commit" /></dd></div><div><dt>Generated</dt><dd>{utc(evidence.generated_at)}</dd></div>{evidence.application.public_origin && <div><dt>Public origin</dt><dd><code>{evidence.application.public_origin}</code></dd></div>}</dl><details><summary>Show bounded diagnostic codes</summary><pre>{`overall=${evidence.overall_state}\npostgres=${evidence.financial_authority.postgres.state}\noutbox=${evidence.delivery_cache.outbox.state}\nworker=${evidence.delivery_cache.outbox.worker_progress}\nredis=${evidence.delivery_cache.redis.state}`}</pre></details><div className="safe-next-action"><strong>Safe next action</strong><p>Run the bounded local status script, then refresh this evidence. A restart cannot prove or reverse a financial result.</p><CopyControl value="powershell -File .\\scripts\\status-local.ps1" label="Copy local status command" /></div></section>
    </section>}
  </>;
}
