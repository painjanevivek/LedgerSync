"use client";

import { ArrowClockwise, LinkSimple } from "@phosphor-icons/react";
import { useCallback, useEffect, useState } from "react";

import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { canSearchInvestigations, deriveConsoleCapabilities } from "@/features/console/capabilities";
import { sanitizeRelatedEvidence, type RelatedEvidence, type RelatedEvidencePage, type RelationshipSourceType, type RelationshipTargetType } from "@/lib/api/related-evidence";
import { Identifier } from "@/ui/display/Identifier";
import { RecordLink } from "@/ui/display/RecordLink";
import { StatePanel } from "@/ui/display/StatePanel";
import { StatusBadge, type StatusTone } from "@/ui/display/StatusBadge";
import { Timestamp } from "@/ui/display/Timestamp";

function targetHref(type: RelationshipTargetType, id: string): string | null {
  switch (type) {
    case "account": return `/accounts/${encodeURIComponent(id)}`;
    case "transfer": return `/transfers/${encodeURIComponent(id)}`;
    case "funding": return `/funding/${encodeURIComponent(id)}`;
    case "event": return `/events/${encodeURIComponent(id)}`;
    case "reconciliation_run": return `/reconciliation/${encodeURIComponent(id)}`;
    case "correction": return `/corrections/${encodeURIComponent(id)}`;
    default: return null;
  }
}

function statusTone(status: string): StatusTone {
  if (["active", "posted", "published", "matched", "approved", "delivered"].includes(status)) return "success";
  if (["rejected", "dead", "failed", "mismatch", "closed", "cancelled", "expired"].includes(status)) return "danger";
  if (["pending", "requested", "retrying", "frozen"].includes(status)) return "warning";
  return "neutral";
}

function RelationshipItem({ item }: Readonly<{ item: RelatedEvidence }>) {
  const href = targetHref(item.target_type, item.target_id);
  return <li className="related-evidence-item">
    <div className="related-evidence-item-heading"><div><p className="eyebrow">{item.relationship_type.replaceAll("_", " ")}</p><h3>{item.safe_label}</h3></div><StatusBadge tone={statusTone(item.status)}>{item.status}</StatusBadge></div>
    <Identifier value={item.target_id} />
    <p className="related-evidence-provenance">PostgreSQL relationship snapshot · <Timestamp value={item.occurred_at} /></p>
    {href ? <RecordLink href={href} label="Open related evidence" /> : <p className="muted">No released detail route. The immutable identifier remains available for an authorized investigation.</p>}
  </li>;
}

export function RelatedEvidenceRail({ sourceType, sourceId }: Readonly<{ sourceType: RelationshipSourceType; sourceId: string }>) {
  const { session, online } = useConsoleSession();
  const canRead = canSearchInvestigations(deriveConsoleCapabilities(session));
  const [page, setPage] = useState<RelatedEvidencePage | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch(`/api/investigation/related/${encodeURIComponent(sourceType)}/${encodeURIComponent(sourceId)}`, { cache: "no-store", signal });
      const sanitized = sanitizeRelatedEvidence(response.status, await response.json() as unknown);
      if (sanitized.status < 200 || sanitized.status >= 300) {
        setPage(null);
        setError(sanitized.status === 403 ? "Your current server-issued authority does not include this relationship view." : sanitized.status === 404 ? "The source is not visible in this authorized tenant scope." : "Current related evidence is unavailable. No missing relationship is inferred.");
        return;
      }
      setPage(sanitized.body as RelatedEvidencePage);
    } catch (cause) {
      if ((cause as Error).name !== "AbortError") {
        setPage(null);
        setError("Current related evidence is unavailable. No missing relationship is inferred.");
      }
    } finally { if (!signal?.aborted) setLoading(false); }
  }, [sourceId, sourceType]);

  useEffect(() => {
    if (!session || !online || !canRead) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => void load(controller.signal), 0);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [canRead, load, online, session]);

  if (!canRead) return null;
  return <section className="related-evidence-rail" aria-labelledby={`related-evidence-${sourceType}-${sourceId}`} aria-busy={loading}>
    <div className="section-heading"><div><p className="eyebrow">Deterministic evidence links</p><h2 id={`related-evidence-${sourceType}-${sourceId}`}>Related evidence</h2><p>Server-derived links only. Financial values remain in their authoritative detail views.</p></div><button className="button secondary" type="button" disabled={!online || loading} onClick={() => void load()}><ArrowClockwise aria-hidden="true" />Refresh links</button></div>
    {!online && !page && <StatePanel kind="offline" title="Related evidence unavailable offline" message="Reconnect to request the current relationship snapshot." />}
    {loading && !page && <StatePanel title="Loading related evidence" message="No relationship is inferred until the bounded server read completes." />}
    {error && <StatePanel kind="unknown" title="Related evidence unavailable" message={error} />}
    {page && <><div className="related-evidence-boundary"><LinkSimple aria-hidden="true" /><span>{page.relationships.length} explicit relationship{page.relationships.length === 1 ? "" : "s"} · generated <Timestamp value={page.generated_at} />{page.truncated ? " · additional links withheld by the bound" : ""}</span></div>{page.relationships.length === 0 ? <StatePanel title="No explicit related evidence" message="No authorized foreign-key or reconciliation-snapshot relationship is recorded. LedgerSync does not manufacture a link from similar timestamps or text." /> : <ol className="related-evidence-list">{page.relationships.map((item) => <RelationshipItem key={`${item.relationship_type}-${item.target_type}-${item.target_id}`} item={item} />)}</ol>}</>}
  </section>;
}
