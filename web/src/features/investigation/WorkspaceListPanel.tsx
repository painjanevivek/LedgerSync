"use client";

import { ArrowClockwise, FolderOpen } from "@phosphor-icons/react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { sanitizeWorkspacePage, type WorkspacePage } from "@/lib/api/investigation-workspaces";
import { StatePanel } from "@/ui/display/StatePanel";
import { StatusBadge } from "@/ui/display/StatusBadge";
import { Timestamp } from "@/ui/display/Timestamp";

export function WorkspaceListPanel() {
  const { session, online } = useConsoleSession(); const canRead = session?.scopes.includes("investigation:read") ?? false;
  const [page, setPage] = useState<WorkspacePage | null>(null); const [loading, setLoading] = useState(false); const [error, setError] = useState(false);
  const load = useCallback(async (signal?: AbortSignal) => {
    if (!session || !online || !canRead) return;
    setLoading(true); setError(false);
    try { const response = await fetch("/api/investigation/workspaces", { cache: "no-store", signal }); const sanitized = sanitizeWorkspacePage(response.status, await response.json().catch(() => ({}))); if (sanitized.status !== 200) { setPage(null); setError(true); return; } setPage(sanitized.body as WorkspacePage); }
    catch (cause) { if ((cause as Error).name !== "AbortError") { setPage(null); setError(true); } }
    finally { if (!signal?.aborted) setLoading(false); }
  }, [canRead, online, session]);
  useEffect(() => { const controller = new AbortController(); const timer = window.setTimeout(() => void load(controller.signal), 0); return () => { window.clearTimeout(timer); controller.abort(); }; }, [load]);
  if (!canRead) return null;
  return <section className="workspace-list-panel" aria-labelledby="workspace-list-heading" aria-busy={loading}>
    <div className="workspace-list-heading"><div><p className="eyebrow">Server-owned case context</p><h2 id="workspace-list-heading"><FolderOpen aria-hidden="true" /> Investigation workspaces</h2></div><button className="button secondary" type="button" disabled={!online || loading} onClick={() => void load()}><ArrowClockwise aria-hidden="true" />{loading ? "Refreshing…" : "Refresh"}</button></div>
    <p>Reopen current evidence with its captured query and immutable lifecycle history. Financial facts are always read live.</p>
    {!online && !page ? <StatePanel kind="offline" title="Investigation workspaces unavailable offline" message="Reconnect to request your current tenant-and-operator-scoped workspace list." /> : null}
    {loading && !page ? <StatePanel title="Loading investigation workspaces" message="No case or ownership is inferred until the server read completes." /> : null}
    {error ? <StatePanel kind="error" title="Investigation list unavailable" message="No missing or handed-off workspace is inferred." action={online ? <button className="button secondary" type="button" onClick={() => void load()}>Retry list</button> : undefined} /> : null}
    {page && page.investigations.length === 0 && !loading ? <StatePanel title="No investigation workspaces" message="Run an exact search, then preserve an authorized result when a case needs repeatable context or handoff." /> : null}
    {page && page.investigations.length > 0 ? <ul className="workspace-summary-list">{page.investigations.map((item) => <li key={item.investigation_id}><div><p className="eyebrow">{item.taxonomy.replaceAll("_", " ")}</p><h3>{item.title}</h3><p className="muted">Updated <Timestamp value={item.updated_at} /> · version {item.version}</p></div><div className="workspace-summary-actions"><StatusBadge tone={item.status === "open" ? "success" : "neutral"}>{item.status}</StatusBadge><Link className="button secondary" href={`/investigations/${item.investigation_id}`}>Open workspace</Link></div></li>)}</ul> : null}
  </section>;
}
