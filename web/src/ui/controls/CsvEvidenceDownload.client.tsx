"use client";

import { useState } from "react";
import { readBoundedBytes } from "@/lib/bounded-stream";

export function CsvEvidenceDownload({ path, filename, label, maxBytes, digestHeader, disabled = false }: Readonly<{
  path: string; filename: string; label: string; maxBytes: number; digestHeader?: string; disabled?: boolean;
}>) {
  const [busy, setBusy] = useState(false), [error, setError] = useState<string | null>(null), [audit, setAudit] = useState<string | null>(null);
  async function download() {
    if (busy || disabled) return;
    setBusy(true); setError(null); setAudit(null);
    try {
      if (!/^\/api\/[A-Za-z0-9/_.-]+$/.test(path)) throw Error("The fixed CSV route is invalid.");
      const response = await fetch(path, { cache: "no-store", signal: AbortSignal.timeout(15000) });
      if (!response.ok || !response.headers.get("content-type")?.startsWith("text/csv")) throw Error("The authorized CSV download is unavailable. No file was saved.");
      const bytes = await readBoundedBytes(response.body, maxBytes), reference = response.headers.get("x-audit-reference");
      if (!reference || !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(reference)) throw Error("The download has no valid audit reference.");
      if (digestHeader) {
        const hash = await crypto.subtle.digest("SHA-256", bytes as BufferSource);
        const actual = [...new Uint8Array(hash)].map(b => b.toString(16).padStart(2,"0")).join("");
        if (actual !== response.headers.get(digestHeader)) throw Error("The original source checksum differs. No file was saved.");
      }
      const url = URL.createObjectURL(new Blob([bytes as BlobPart], { type: "text/csv;charset=utf-8" })), anchor = document.createElement("a");
      anchor.href = url; anchor.download = filename; anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000); setAudit(reference);
    } catch (cause) { setError(cause instanceof Error ? cause.message : "The download failed."); }
    finally { setBusy(false); }
  }
  return <span><button className="button secondary" type="button" disabled={disabled || busy} aria-busy={busy} onClick={() => void download()}>{busy ? "Preparing CSV…" : label}</button>{error && <span role="alert">{error}</span>}{audit && <span role="status"> CSV prepared. Audit reference: <code>{audit}</code></span>}</span>;
}
