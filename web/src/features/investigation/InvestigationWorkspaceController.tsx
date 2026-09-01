"use client";

import { ArrowClockwise, ArrowsLeftRight, LockSimple, LockSimpleOpen } from "@phosphor-icons/react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { sanitizeWorkspace, sanitizeWorkspaceReceipt, type InvestigationWorkspace, type WorkspaceReceipt } from "@/lib/api/investigation-workspaces";
import { Identifier } from "@/ui/display/Identifier";
import { PageHeader } from "@/ui/display/PageHeader";
import { RecordLink } from "@/ui/display/RecordLink";
import { StatePanel } from "@/ui/display/StatePanel";
import { StatusBadge, type StatusTone } from "@/ui/display/StatusBadge";
import { Timestamp } from "@/ui/display/Timestamp";
import { FormField } from "@/ui/forms/FormField.client";

function tone(status: string): StatusTone { if (["active", "posted", "published", "matched", "approved", "succeeded", "open"].includes(status)) return "success"; if (["rejected", "dead", "failed", "mismatch", "denied", "closed", "cancelled", "expired"].includes(status)) return "danger"; if (["pending", "requested", "retrying", "frozen"].includes(status)) return "warning"; return "neutral"; }
function currentPath(type: string, id: string) { switch (type) { case "account": return `/accounts/${id}`; case "transfer": return `/transfers/${id}`; case "funding": return `/funding/${id}`; case "event": return `/events/${id}`; case "reconciliation_run": return `/reconciliation/${id}`; case "correction": return `/corrections/${id}`; default: return ""; } }

export function InvestigationWorkspaceController({ investigationId, invalidId }: Readonly<{ investigationId: string; invalidId: boolean }>) {
  const { session, online } = useConsoleSession(); const canRead = session?.scopes.includes("investigation:read") ?? false; const canWrite = session?.scopes.includes("investigation:write") ?? false;
  const [workspace, setWorkspace] = useState<InvestigationWorkspace | null>(null); const [loading, setLoading] = useState(false); const [error, setError] = useState<string | null>(null); const [pending, setPending] = useState<string | null>(null); const [targetSubject, setTargetSubject] = useState(""); const [handoffComplete, setHandoffComplete] = useState(false);
  const load = useCallback(async (signal?: AbortSignal) => {
    if (!session || !online || !canRead || invalidId || handoffComplete) return;
    setLoading(true); setError(null);
    try { const response = await fetch(`/api/investigation/workspaces/${encodeURIComponent(investigationId)}`, { cache: "no-store", signal }); const sanitized = sanitizeWorkspace(response.status, await response.json().catch(() => ({}))); if (sanitized.status !== 200) { setWorkspace(null); setError(sanitized.status === 404 ? "This workspace is unavailable to the current tenant, operator, or record scopes. No existence or prior ownership is inferred." : "Investigation evidence is temporarily unavailable."); return; } setWorkspace(sanitized.body as InvestigationWorkspace); }
    catch (cause) { if ((cause as Error).name !== "AbortError") { setWorkspace(null); setError("Investigation evidence is temporarily unavailable."); } }
    finally { if (!signal?.aborted) setLoading(false); }
  }, [canRead, handoffComplete, investigationId, invalidId, online, session]);
  useEffect(() => { const controller = new AbortController(); const timer = window.setTimeout(() => void load(controller.signal), 0); return () => { window.clearTimeout(timer); controller.abort(); }; }, [load]);

  async function mutate(action: "handoff" | "close" | "reopen") {
    if (!workspace || !session || pending) return;
    setPending(action); setError(null);
    try {
      const response = await fetch(`/api/investigation/workspaces/${workspace.investigation_id}/${action}`, { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": session.csrf_token }, body: JSON.stringify(action === "handoff" ? { expected_version: workspace.version, target_subject_id: targetSubject.trim() } : { expected_version: workspace.version }) });
      const sanitized = sanitizeWorkspaceReceipt(response.status, await response.json().catch(() => ({})));
      if (sanitized.status !== 200) { setError(sanitized.status === 409 ? "The workspace changed in another session. Refresh before retrying." : "The lifecycle change was not verified and no ownership or status change is inferred."); return; }
      const receipt = sanitized.body as WorkspaceReceipt;
      if (receipt.outcome === "handed_off") { setHandoffComplete(true); setWorkspace(null); return; }
      await load();
    } catch { setError("The lifecycle change was not verified. Retry only after refreshing current workspace evidence."); }
    finally { setPending(null); }
  }

  return <ConsoleRouteFrame section="search" loadingLabel="Investigation workspace" pending={loading || pending !== null}>
    <div className="investigation-workspace-detail">
      <PageHeader eyebrow="Investigate / Preserved workspace" title={workspace?.title ?? "Investigation workspace"} description="Historical query and relationship context is separated from current PostgreSQL evidence. No balance, amount, payload, secret, or free-form note is stored here." />
      {invalidId ? <StatePanel kind="error" title="Invalid investigation URL" message="The workspace identifier is malformed. No protected request was made." action={<Link className="button secondary" href="/search">Return to search</Link>} /> : !canRead && session ? <StatePanel kind="denied" title="Investigation authority required" message="Your server-issued scopes do not permit this workspace read." /> : handoffComplete ? <StatePanel title="Investigation handed off" message="Ownership changed atomically. This operator no longer reads the workspace; the recipient must reopen it with their own current tenant and record scopes." action={<Link className="button secondary" href="/search">Return to investigations</Link>} /> : <>
        {!online && !workspace ? <StatePanel kind="offline" title="Workspace unavailable offline" message="Reconnect to reauthorize the workspace and load current evidence." /> : null}
        {loading && !workspace ? <StatePanel title="Loading authorized investigation" message="Historical context and live evidence remain unconfirmed until the server read completes." /> : null}
        {error ? <StatePanel kind="error" title="Investigation change not verified" message={error} action={online ? <button className="button secondary" type="button" onClick={() => void load()}>Refresh workspace</button> : undefined} /> : null}
        {workspace ? <>
          <section className="workspace-overview" aria-labelledby="workspace-overview-heading"><div className="workspace-overview-heading"><div><p className="eyebrow">Case state</p><h2 id="workspace-overview-heading">Workspace overview</h2></div><StatusBadge tone={tone(workspace.status)}>{workspace.status}</StatusBadge></div><dl className="workspace-facts"><div><dt>Taxonomy</dt><dd>{workspace.taxonomy.replaceAll("_", " ")}</dd></div><div><dt>Version</dt><dd>{workspace.version}</dd></div><div><dt>Created UTC</dt><dd><Timestamp value={workspace.created_at} /></dd></div><div><dt>Updated UTC</dt><dd><Timestamp value={workspace.updated_at} /></dd></div></dl></section>
          <section className="workspace-current" aria-labelledby="workspace-current-heading"><div className="section-heading"><div><p className="eyebrow">Live reauthorization boundary</p><h2 id="workspace-current-heading">Current evidence</h2><p>Statuses and links below were re-read now; they are not the captured investigation snapshot.</p></div><button className="button secondary" type="button" disabled={!online || loading} onClick={() => void load()}><ArrowClockwise aria-hidden="true" />Refresh current evidence</button></div>
            {!workspace.current_evidence.available || !workspace.current_evidence.root ? <StatePanel kind="unknown" title="Current root evidence unavailable" message="No financial state is inferred. The record may be unavailable under current authorization or evidence may be temporarily unreadable." /> : <div className="workspace-current-root"><div><p className="eyebrow">{workspace.current_evidence.root.record_type.replaceAll("_", " ")}</p><h3>{workspace.current_evidence.root.safe_label}</h3><p><Identifier value={workspace.current_evidence.root.record_id} /></p></div><StatusBadge tone={tone(workspace.current_evidence.root.status)}>{workspace.current_evidence.root.status}</StatusBadge></div>}
            {workspace.current_evidence.relationships.length > 0 ? <ul className="workspace-current-relationships">{workspace.current_evidence.relationships.map((item) => { const path = currentPath(item.target_type, item.target_id); return <li key={`${item.relationship_type}:${item.target_type}:${item.target_id}`}><div><p className="eyebrow">{item.relationship_type.replaceAll("_", " ")}</p><h3>{item.safe_label}</h3><p className="muted"><Timestamp value={item.occurred_at} /> · PostgreSQL relationship snapshot</p></div><div><StatusBadge tone={tone(item.status)}>{item.status}</StatusBadge>{path ? <RecordLink href={path} label="Open current detail" /> : null}</div></li>; })}</ul> : null}
            <p className="muted">Generated <Timestamp value={workspace.current_evidence.generated_at} />{workspace.current_evidence.truncated ? " · More relationships were withheld by the response bound." : ""}</p>
          </section>
          <section className="workspace-history" aria-labelledby="workspace-history-heading"><div><p className="eyebrow">Preserved navigation context</p><h2 id="workspace-history-heading">Historical investigation context</h2><p>This section records what was used to create and route the case. It does not claim those records still have the same status.</p></div><dl className="workspace-facts"><div><dt>Query kind</dt><dd>{workspace.historical_context.query_context.kind.replaceAll("_", " ")}</dd></div><div><dt>Query value</dt><dd><Identifier value={workspace.historical_context.query_context.value} /></dd></div><div><dt>Root type</dt><dd>{workspace.historical_context.query_context.record_type.replaceAll("_", " ")}</dd></div><div><dt>Withheld references</dt><dd>{workspace.historical_context.withheld_reference_count}</dd></div></dl>
            <details><summary>Captured authorized references ({workspace.historical_context.references.length})</summary><ul className="workspace-reference-list">{workspace.historical_context.references.map((item) => <li key={`${item.record_type}:${item.record_id}`}><div><strong>{item.relationship_type.replaceAll("_", " ")}</strong><span>{item.record_type.replaceAll("_", " ")} · <Identifier value={item.record_id} /></span></div>{item.target_path ? <RecordLink href={item.target_path} label="Open current detail" /> : <span className="muted">No released detail route</span>}</li>)}</ul></details>
            <ol className="workspace-timeline">{workspace.historical_context.history.map((item) => <li key={`${item.version}:${item.action}`}><span aria-hidden="true" /><div><strong>{item.action.replaceAll("_", " ")}</strong><p>{item.actor_is_current_operator ? "Current operator" : "Another operator"} · workspace {item.status} · version {item.version}</p><Timestamp value={item.occurred_at} /></div></li>)}</ol>{workspace.historical_context.history_truncated ? <p className="muted">Older lifecycle entries are retained server-side but omitted from this bounded response.</p> : null}
          </section>
          {canWrite ? <section className="workspace-lifecycle" aria-labelledby="workspace-lifecycle-heading"><div><p className="eyebrow">Explicit ownership and lifecycle</p><h2 id="workspace-lifecycle-heading">Manage workspace</h2><p>These actions change case context only. They never mutate ledger records or current financial evidence.</p></div><div className="workspace-lifecycle-actions">{workspace.status === "open" ? <button className="button secondary" type="button" disabled={!online || pending !== null} onClick={() => void mutate("close")}><LockSimple aria-hidden="true" />{pending === "close" ? "Closing…" : "Close investigation"}</button> : <button className="button secondary" type="button" disabled={!online || pending !== null} onClick={() => void mutate("reopen")}><LockSimpleOpen aria-hidden="true" />{pending === "reopen" ? "Reopening…" : "Reopen investigation"}</button>}</div>{workspace.status === "open" ? <details className="workspace-handoff"><summary><ArrowsLeftRight aria-hidden="true" /> Hand off to another operator</summary><div><p>Ownership will move atomically. The recipient still needs the same tenant and current record-read scopes. Enter the exact server-issued subject ID; names and email addresses are not accepted as notes.</p><FormField label="Recipient subject ID" requirement="required" hint="Example: operator-2"><input value={targetSubject} onChange={(event) => { setTargetSubject(event.target.value); setError(null); }} maxLength={255} autoComplete="off" spellCheck={false} /></FormField><button className="button danger guarded-control" type="button" disabled={!online || pending !== null || !targetSubject.trim() || targetSubject.trim() === session?.subject_id} onClick={() => void mutate("handoff")}>{pending === "handoff" ? "Handing off…" : "Confirm ownership handoff"}</button></div></details> : null}</section> : null}
        </> : null}
      </>}
    </div>
  </ConsoleRouteFrame>;
}
