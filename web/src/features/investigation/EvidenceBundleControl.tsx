"use client";

import { DownloadSimple, FileZip, ShieldWarning } from "@phosphor-icons/react";
import { useEffect, useId, useRef, useState } from "react";

type BundleState = "idle" | "generating" | "complete" | "error";

function filename(disposition: string | null) {
  const match = disposition?.match(/^attachment; filename="(ledgersync-investigation-[0-9a-f-]+-\d{8}T\d{6}Z-v1\.zip)"$/u);
  return match?.[1] ?? null;
}

export function EvidenceBundleControl({ investigationId, version, historicalReferenceCount, currentEvidenceCount, csrfToken, online, canExport }: Readonly<{ investigationId: string; version: string; historicalReferenceCount: number; currentEvidenceCount: number; csrfToken: string; online: boolean; canExport: boolean }>) {
  const dialog = useRef<HTMLDialogElement>(null); const heading = useRef<HTMLHeadingElement>(null); const trigger = useRef<HTMLButtonElement>(null); const result = useRef<HTMLHeadingElement>(null); const descriptionId = `bundle-${useId().replace(/[^a-z0-9]/giu, "")}`;
  const [state, setState] = useState<BundleState>("idle"); const [message, setMessage] = useState("");
  const busy = state === "generating";
  useEffect(() => { if (state === "complete" || state === "error") result.current?.focus(); }, [state]);

  function open() { setState("idle"); setMessage(""); dialog.current?.showModal(); window.requestAnimationFrame(() => heading.current?.focus()); }
  async function generate() {
    dialog.current?.close(); setState("generating"); setMessage("");
    try {
      const response = await fetch(`/api/investigation/workspaces/${encodeURIComponent(investigationId)}/evidence-bundle`, { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken }, body: JSON.stringify({ expected_version: version }), cache: "no-store" });
      if (!response.ok) { const value = await response.json().catch(() => ({})) as { error?: { code?: string } }; throw new Error(value.error?.code === "investigation_version_conflict" ? "The workspace changed after this scope review. Refresh it before generating another bundle." : "The server did not produce an audited evidence bundle. No download was saved."); }
      const safeName = filename(response.headers.get("content-disposition")); const declared = response.headers.get("content-length");
      if (!safeName || response.headers.get("content-type") !== "application/zip" || response.headers.get("x-ledgersync-bundle-schema") !== "1" || !declared || !/^\d+$/u.test(declared) || Number(declared) > 512 * 1024) throw new Error("The bundle response failed browser safety checks. No download was saved.");
      const blob = await response.blob(); if (blob.size !== Number(declared) || blob.size > 512 * 1024) throw new Error("The bundle size did not match its reviewed response. No download was saved.");
      const url = URL.createObjectURL(blob); const anchor = document.createElement("a"); anchor.href = url; anchor.download = safeName; document.body.append(anchor); anchor.click(); anchor.remove(); window.setTimeout(() => URL.revokeObjectURL(url), 0);
      setState("complete"); setMessage("The audited ZIP was handed to the browser. Treat every file as a historical snapshot and reopen this workspace before making a current-state decision.");
    } catch (cause) { setState("error"); setMessage(cause instanceof Error ? cause.message : "The evidence bundle was not generated."); }
  }

  return <section className="workspace-bundle" aria-labelledby={`${descriptionId}-section-heading`}>
    <div><p className="eyebrow">Bounded offline evidence</p><h2 id={`${descriptionId}-section-heading`}>Evidence bundle</h2><p>Generate a one-time, server-audited ZIP after reviewing its exact identifier-only scope.</p></div>
    <button ref={trigger} className="button secondary" type="button" onClick={open} disabled={!online || !canExport || busy}><FileZip aria-hidden="true" />{busy ? "Generating bundle…" : "Review evidence bundle"}</button>
    {!canExport ? <p className="export-permission-note">Bundle download requires the authorized exports:read scope.</p> : null}
    {state !== "idle" && state !== "generating" ? <div className="export-progress" role={state === "error" ? "alert" : "status"} aria-live="polite"><h3 ref={result} tabIndex={-1}>{state === "complete" ? "Bundle download started" : "Bundle not generated"}</h3><p>{message}</p>{state === "error" ? <button className="button secondary" type="button" onClick={open}>Review and retry</button> : null}</div> : null}
    <dialog ref={dialog} className="confirmation-dialog export-review-dialog" aria-labelledby={`${descriptionId}-heading`} aria-describedby={`${descriptionId}-description`} onClose={() => { if (!busy) trigger.current?.focus(); }}>
      <div><p className="eyebrow">Exact evidence scope</p><h2 ref={heading} tabIndex={-1} id={`${descriptionId}-heading`}>Review investigation bundle</h2><p id={`${descriptionId}-description`}>The server will reauthorize workspace version {version}, generate the files below, record the download, and discard its copy after the response.</p>
        <div className="export-review-proof"><div><span>Workspace reference</span><strong>{investigationId}</strong></div><div><span>Historical references</span><strong>{historicalReferenceCount} identifier row{historicalReferenceCount === 1 ? "" : "s"}</strong></div><div><span>Current evidence</span><strong>{currentEvidenceCount} root/relationship row{currentEvidenceCount === 1 ? "" : "s"}</strong></div><div><span>Archive files</span><strong>Manifest JSON + 3 UTF-8 CSV files</strong></div><div><span>Integrity</span><strong>SHA-256 per CSV and for the complete ZIP</strong></div><div><span>Expiry</span><strong>15 minutes · server retention ends with the response</strong></div></div>
        <div className="export-not-backup"><ShieldWarning weight="fill" aria-hidden="true" /><p><strong>This is not live financial authority.</strong> Amounts, balances, payloads, labels, notes, tenant/operator names, and credentials are excluded. Reopen the workspace to verify current state.</p></div>
        <div className="action-row"><button className="button secondary guarded-control" type="button" onClick={() => dialog.current?.close()}>Cancel</button><button className="button primary guarded-control" type="button" onClick={() => void generate()}><DownloadSimple aria-hidden="true" />Generate audited ZIP</button></div>
      </div>
    </dialog>
  </section>;
}
