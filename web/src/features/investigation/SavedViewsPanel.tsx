"use client";

import { ArrowSquareOut, BookmarkSimple, PencilSimple, Trash } from "@phosphor-icons/react";
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";

import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { sanitizeSavedView, sanitizeSavedViewPage, type SavedInvestigationView, type SavedInvestigationViewPage } from "@/lib/api/saved-investigation-views";
import { StatePanel } from "@/ui/display/StatePanel";
import { Timestamp } from "@/ui/display/Timestamp";
import { FormField } from "@/ui/forms/FormField.client";
import { ConfirmationDialog } from "@/ui/overlays/ConfirmationDialog.client";

function filterSummary(view: SavedInvestigationView) {
  return Object.entries(view.filters).map(([key, value]) => `${key.replaceAll("_", " ")}: ${value.replaceAll("_", " ")}`).join(" · ");
}

export function SavedViewsPanel() {
  const { session, online } = useConsoleSession();
  const canRead = session?.scopes.includes("investigation:read") ?? false;
  const canWrite = session?.scopes.includes("investigation:write") ?? false;
  const [page, setPage] = useState<SavedInvestigationViewPage | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busyID, setBusyID] = useState<string | null>(null);
  const [renameID, setRenameID] = useState<string | null>(null);
  const [renameName, setRenameName] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<SavedInvestigationView | null>(null);
  const deleteTrigger = useRef<HTMLButtonElement | null>(null);

  const load = useCallback(async (signal?: AbortSignal) => {
    if (!session || !online || !canRead) return;
    setLoading(true); setError(null);
    try {
      const response = await fetch("/api/investigation/saved-views", { cache: "no-store", signal });
      const sanitized = sanitizeSavedViewPage(response.status, await response.json().catch(() => ({})));
      if (sanitized.status !== 200) { setPage(null); setError("Saved views are unavailable. No missing view is inferred."); return; }
      setPage(sanitized.body as SavedInvestigationViewPage);
    } catch (cause) {
      if ((cause as Error).name !== "AbortError") { setPage(null); setError("Saved views are unavailable. No missing view is inferred."); }
    } finally { if (!signal?.aborted) setLoading(false); }
  }, [canRead, online, session]);

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => void load(controller.signal), 0);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [load]);

  async function rename(view: SavedInvestigationView) {
    if (!session || busyID || !renameName.trim()) return;
    setBusyID(view.saved_view_id); setError(null);
    try {
      const response = await fetch(`/api/investigation/saved-views/${encodeURIComponent(view.saved_view_id)}`, { method: "PUT", headers: { "Content-Type": "application/json", "X-CSRF-Token": session.csrf_token }, body: JSON.stringify({ expected_version: view.version, name: renameName.trim() }) });
      const sanitized = sanitizeSavedView(response.status, await response.json().catch(() => ({})));
      if (sanitized.status !== 200) { setError(sanitized.status === 409 ? "The saved view changed or its name conflicts. The latest list has been reloaded." : "The saved view was not renamed."); if (sanitized.status === 409) await load(); return; }
      const updated = sanitized.body as SavedInvestigationView;
      setPage((current) => current ? { ...current, views: current.views.map((item) => item.saved_view_id === updated.saved_view_id ? updated : item) } : current);
      setRenameID(null); setRenameName("");
    } catch { setError("The saved view was not renamed because the request could not be verified."); }
    finally { setBusyID(null); }
  }

  async function remove() {
    if (!session || !deleteTarget || busyID) return;
    setBusyID(deleteTarget.saved_view_id); setError(null);
    try {
      const response = await fetch(`/api/investigation/saved-views/${encodeURIComponent(deleteTarget.saved_view_id)}`, { method: "DELETE", headers: { "X-CSRF-Token": session.csrf_token, "If-Match": `"${deleteTarget.version}"` } });
      if (response.status !== 204) { setError(response.status === 409 ? "The saved view changed in another session. The latest list has been reloaded." : "The saved view was not deleted."); if (response.status === 409) await load(); return; }
      setPage((current) => current ? { ...current, views: current.views.filter((item) => item.saved_view_id !== deleteTarget.saved_view_id) } : current);
      setDeleteTarget(null);
    } catch { setError("The saved view was not deleted because the request could not be verified."); }
    finally { setBusyID(null); }
  }

  if (!canRead) return null;
  return <section className="saved-views-panel" aria-labelledby="saved-views-heading" aria-busy={loading}>
    <div className="saved-views-heading"><div><p className="eyebrow">Current evidence shortcuts</p><h2 id="saved-views-heading"><BookmarkSimple aria-hidden="true" /> Saved operational views</h2></div><button className="button secondary" type="button" disabled={!online || loading} onClick={() => void load()}>{loading ? "Refreshing…" : "Refresh views"}</button></div>
    <p>Each shortcut stores only an allowlisted filter definition. Opening it runs the current authorized query; it never replays a result snapshot.</p>
    {!online && !page ? <StatePanel kind="offline" title="Saved views unavailable offline" message="Reconnect to load server-owned shortcuts. No empty list is inferred." /> : null}
    {loading && !page ? <StatePanel title="Loading saved views" message="No view is inferred until the tenant-and-operator-scoped read completes." /> : null}
    {error ? <StatePanel kind="error" title="Saved-view change not verified" message={error} action={online ? <button className="button secondary" type="button" onClick={() => void load()}>Reload saved views</button> : undefined} /> : null}
    {page && page.views.length === 0 && !loading ? <StatePanel title="No saved views yet" message="Apply a structured filter on Accounts, Transfers, Funding, Approvals, Corrections, Events, or Webhooks, then choose Save current filters." /> : null}
    {page && page.views.length > 0 ? <ul className="saved-view-list">{page.views.map((view) => <li key={view.saved_view_id} className="saved-view-card">
      <div className="saved-view-card-heading"><div><p className="eyebrow">{view.domain}</p><h3>{view.name}</h3></div><Link className="button secondary" href={view.target_path}>Open current evidence <ArrowSquareOut aria-hidden="true" /></Link></div>
      <p>{filterSummary(view)}</p>
      <p className="muted">Schema v{view.filter_schema_version} · view v{view.version} · updated <Timestamp value={view.updated_at} /></p>
      {canWrite ? <div className="saved-view-actions">
        <button className="button tertiary" type="button" disabled={busyID !== null} onClick={() => { setRenameID(view.saved_view_id); setRenameName(view.name); setError(null); }}><PencilSimple aria-hidden="true" /> Rename</button>
        <button ref={(element) => { if (deleteTarget?.saved_view_id === view.saved_view_id) deleteTrigger.current = element; }} className="button danger guarded-control" type="button" disabled={busyID !== null} onClick={(event) => { deleteTrigger.current = event.currentTarget; setDeleteTarget(view); }}><Trash aria-hidden="true" /> Delete</button>
      </div> : null}
      {renameID === view.saved_view_id ? <form className="saved-view-rename" onSubmit={(event) => { event.preventDefault(); void rename(view); }}>
        <FormField label="New saved view name" requirement="required" hint="Do not include customer data or credentials."><input autoFocus value={renameName} onChange={(event) => setRenameName(event.target.value)} maxLength={80} disabled={busyID !== null} /></FormField>
        <div className="action-row"><button className="button secondary" type="button" disabled={busyID !== null} onClick={() => { setRenameID(null); setRenameName(""); }}>Cancel</button><button className="button primary" type="submit" disabled={busyID !== null || !renameName.trim()}>{busyID === view.saved_view_id ? "Renaming…" : "Save name"}</button></div>
      </form> : null}
    </li>)}</ul> : null}
    <ConfirmationDialog open={deleteTarget !== null} eyebrow="Saved preference" title="Delete this saved view?" description="This removes only your saved filter shortcut. It does not delete or change any ledger, delivery, or reconciliation evidence." confirmLabel="Delete saved view" busyLabel="Deleting…" busy={deleteTarget !== null && busyID === deleteTarget.saved_view_id} returnFocusRef={deleteTrigger} onDismiss={() => setDeleteTarget(null)} onConfirm={() => void remove()}>
      {deleteTarget ? <div className="review-summary"><p><strong>Name</strong><span>{deleteTarget.name}</span></p><p><strong>Domain</strong><span>{deleteTarget.domain}</span></p><p><strong>Version</strong><span>{deleteTarget.version}</span></p></div> : null}
    </ConfirmationDialog>
  </section>;
}
