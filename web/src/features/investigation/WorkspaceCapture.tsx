"use client";

import { FolderPlus } from "@phosphor-icons/react";
import { useRouter } from "next/navigation";
import { useState } from "react";

import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { parseWorkspaceCreateInput, sanitizeWorkspace, workspaceTaxonomies, type InvestigationWorkspace, type WorkspaceRecordType, type WorkspaceTaxonomy } from "@/lib/api/investigation-workspaces";
import type { InvestigationSearchResult } from "@/lib/api/investigation-search";
import { FormField } from "@/ui/forms/FormField.client";

function rootRecord(result: InvestigationSearchResult): Readonly<{ type: WorkspaceRecordType; id: string }> | null {
  if (result.record_type === "request_reference") return result.related_record_type && result.related_record_id ? { type: result.related_record_type, id: result.related_record_id } : null;
  if (result.record_type === "account" || result.record_type === "transfer" || result.record_type === "funding" || result.record_type === "event" || result.record_type === "reconciliation_run" || result.record_type === "reconciliation_mismatch" || result.record_type === "correction") return { type: result.record_type, id: result.record_id };
  return null;
}

function suggestedTaxonomy(recordType: WorkspaceRecordType): WorkspaceTaxonomy {
  if (recordType === "account") return "account_state";
  if (recordType === "transfer" || recordType === "event") return "transfer_delivery";
  if (recordType === "funding") return "funding";
  if (recordType === "reconciliation_run" || recordType === "reconciliation_mismatch") return "reconciliation";
  if (recordType === "correction") return "correction";
  return "other";
}

export function WorkspaceCapture({ result, query, queryKind }: Readonly<{ result: InvestigationSearchResult; query: string; queryKind: "immutable_id" | "approved_reference" }>) {
  const { session, online } = useConsoleSession(); const router = useRouter(); const root = rootRecord(result);
  const [title, setTitle] = useState(`${result.safe_label} review`.slice(0, 80));
  const [taxonomy, setTaxonomy] = useState<WorkspaceTaxonomy>(root ? suggestedTaxonomy(root.type) : "other");
  const [pending, setPending] = useState(false); const [error, setError] = useState<string | null>(null);
  if (!root || !session?.scopes.includes("investigation:write")) return null;

  async function create() {
    if (!session || pending || !root) return;
    setPending(true); setError(null);
    try {
      const input = parseWorkspaceCreateInput({ title, taxonomy, query_context: { kind: queryKind, record_type: root.type, value: query }, root_record: { record_type: root.type, record_id: root.id } });
      const response = await fetch("/api/investigation/workspaces", { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": session.csrf_token }, body: JSON.stringify(input) });
      const sanitized = sanitizeWorkspace(response.status, await response.json().catch(() => ({})));
      if (sanitized.status !== 201) { setError(sanitized.status === 409 ? "The open-investigation limit was reached. Close an active workspace before creating another." : "The workspace was not created. Current authorization and evidence could not be verified."); return; }
      router.push(`/investigations/${encodeURIComponent((sanitized.body as InvestigationWorkspace).investigation_id)}`);
    } catch { setError("Use a short operational title without customer data, email addresses, URLs, credentials, or secret-like text."); }
    finally { setPending(false); }
  }

  return <details className="workspace-capture">
    <summary><FolderPlus aria-hidden="true" /> Preserve as investigation</summary>
    <div className="workspace-capture-body">
      <p>The server preserves this query, the authorized root ID, and bounded relationship references. It does not copy financial facts or free-form content.</p>
      <FormField label="Safe investigation title" requirement="required" hint="Use an operational label only. Personal, financial, contact, and access information is rejected."><input value={title} onChange={(event) => { setTitle(event.target.value); setError(null); }} maxLength={80} autoComplete="off" /></FormField>
      <FormField label="Taxonomy" requirement="required" hint="A fixed category helps recipients understand why the workspace exists."><select value={taxonomy} onChange={(event) => setTaxonomy(event.target.value as WorkspaceTaxonomy)}>{workspaceTaxonomies.map((value) => <option key={value} value={value}>{value.replaceAll("_", " ")}</option>)}</select></FormField>
      {error ? <p className="field-error" role="alert">{error}</p> : null}
      <button className="button primary" type="button" disabled={!online || pending || !title.trim()} onClick={() => void create()}>{pending ? "Creating workspace…" : "Create investigation workspace"}</button>
    </div>
  </details>;
}
