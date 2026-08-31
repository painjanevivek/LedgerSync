"use client";

import { BookmarkSimple } from "@phosphor-icons/react";
import { useState } from "react";

import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { createSavedViewInput, sanitizeSavedView, type SavedViewDomain } from "@/lib/api/saved-investigation-views";
import { FormField } from "@/ui/forms/FormField.client";

export function SavedViewCapture({ domain, filters }: Readonly<{ domain: SavedViewDomain; filters: Readonly<Record<string, string | undefined>> }>) {
  const { session, online } = useConsoleSession();
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState<string | null>(null);
  const currentFilters = Object.fromEntries(Object.entries(filters).filter((entry): entry is [string, string] => typeof entry[1] === "string" && entry[1] !== ""));
  let definition: ReturnType<typeof createSavedViewInput> | null = null;
  try { definition = createSavedViewInput(name || "Current view", domain, currentFilters); } catch { definition = null; }
  const canWrite = session?.scopes.includes("investigation:write") ?? false;

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!session || !online || !canWrite || busy) return;
    let input: ReturnType<typeof createSavedViewInput>;
    try { input = createSavedViewInput(name.trim(), domain, currentFilters); }
    catch {
      setError("Use a 1–80 character name and at least one structured filter. Free-text search and page cursors are intentionally not saved.");
      return;
    }
    setBusy(true); setError(null); setSaved(null);
    try {
      const response = await fetch("/api/investigation/saved-views", { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": session.csrf_token }, body: JSON.stringify(input) });
      const sanitized = sanitizeSavedView(response.status, await response.json().catch(() => ({})));
      if (sanitized.status !== 201) {
        const code = "error" in sanitized.body ? sanitized.body.error.code : "evidence_unavailable";
        setError(code === "saved_view_name_conflict" ? "That saved-view name is already in use." : code === "saved_view_limit_reached" ? "You have reached the 25-view limit. Delete an old view before saving another." : "The view was not saved. Current evidence and filters were not changed.");
        return;
      }
      setSaved(name.trim());
      setName("");
    } catch {
      setError("The view was not saved because the request could not be verified.");
    } finally { setBusy(false); }
  }

  if (!session?.scopes.includes("investigation:read")) return null;
  return <details className="saved-view-capture">
    <summary><BookmarkSimple aria-hidden="true" /> Save current filters</summary>
    <div className="saved-view-capture-body">
      <p>Stores this structured filter definition for your tenant and operator identity. Results, balances, free-text searches, and page cursors are never saved.</p>
      {!canWrite ? <p className="permission-note">Your server-issued session can open saved views but cannot create or change them.</p> : !definition ? <p className="muted">Apply at least one status, category, date, or approved identifier filter before saving this view.</p> : null}
      {canWrite && <form onSubmit={submit}>
        <FormField label="Saved view name" requirement="required" hint="A short operational label; do not include customer data or credentials." error={error}>
          <input value={name} onChange={(event) => { setName(event.target.value); setError(null); setSaved(null); }} maxLength={80} autoComplete="off" disabled={!definition || busy} placeholder="Example: Dead events this week" />
        </FormField>
        <button className="button secondary" type="submit" disabled={!definition || !online || busy || !name.trim()}>{busy ? "Saving…" : "Save view"}</button>
      </form>}
      {saved ? <p className="saved-view-success" role="status">Saved “{saved}”. Open or manage it from Investigation search.</p> : null}
    </div>
  </details>;
}
