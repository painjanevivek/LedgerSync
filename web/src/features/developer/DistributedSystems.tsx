"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import type { DistributedEvidence } from "@/lib/api/distributed";
import { readJSON, unavailableMessage } from "@/lib/api/client";
import { StatePanel } from "@/ui/display/StatePanel";
import { StatusBadge } from "@/ui/display/StatusBadge";
import { Timestamp } from "@/ui/display/Timestamp";
import { CopyControl } from "@/ui/controls/CopyControl.client";
export function DistributedSystems({ online, canRead }: Readonly<{ online: boolean; canRead: boolean }>) {
  const [evidence, setEvidence] = useState<DistributedEvidence | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [expiredSample, setExpiredSample] = useState<string | null>(null);
  const generation = useRef(0);
  const refresh = useCallback(async () => {
    if (!online || !canRead) return;
    const current = ++generation.current;
    setBusy(true);
    const response = await readJSON<DistributedEvidence>("/api/developer/distributed");
    if (current !== generation.current) return;
    if (response.ok) { setEvidence(response.data); setError(null); }
    else setError(unavailableMessage(response.status, "distributed system observations", response.requestReference));
    setBusy(false);
  }, [online, canRead]);
  useEffect(() => { const timer = window.setTimeout(() => void refresh(), 0); return () => { window.clearTimeout(timer); generation.current += 1; }; }, [refresh]);
  useEffect(() => {
    const scheduler = evidence?.scheduler;
    if (!scheduler?.expires_at || scheduler.state !== "active") return;
    const remaining = Math.max(0, Date.parse(scheduler.expires_at) - Date.parse(scheduler.sampled_at));
    const timer = window.setTimeout(() => setExpiredSample(scheduler.sampled_at), remaining);
    return () => window.clearTimeout(timer);
  }, [evidence]);
  const historical = Boolean(error) || !online;
  const schedulerHistorical = historical || expiredSample === evidence?.scheduler.sampled_at;
  return <section aria-label="Distributed system observations">
    <div className="section-heading"><h2>Service observations</h2><button type="button" className="button secondary" onClick={() => void refresh()} disabled={busy || !online || !canRead}>{busy ? "Refreshing observations…" : "Refresh observations"}</button></div>
    {!canRead && <StatePanel kind="denied" title="Observations not authorized" message="This session needs developer:read." />}
    {!online && <StatePanel kind="offline" title="Offline" message="Reconnect to refresh observations." />}
    {error && <StatePanel kind="error" title="Observations unavailable" message={error} />}
    {busy && !evidence && <p role="status">Loading observations…</p>}
    {evidence && <>
      {historical && <StatusBadge tone="warning">Historical observations</StatusBadge>}
      <dl className="evidence-list">
        <div><dt>Ready API replicas</dt><dd>{evidence.api_replicas.count ?? "Unavailable"}<br /><Timestamp value={evidence.api_replicas.sampled_at} /></dd></div>
        <div><dt>Historical publication evidence</dt><dd>{evidence.worker_publications.count === undefined ? "Unavailable" : `${evidence.worker_publications.count} worker identities in the latest 1,000 published attempts`}<br /><Timestamp value={evidence.worker_publications.sampled_at} />{evidence.worker_publications.latest_published_at && <><br />Latest publication: <Timestamp value={evidence.worker_publications.latest_published_at} /></>}</dd></div>
        <div><dt>Active scheduler</dt><dd><StatusBadge tone="neutral">{evidence.scheduler.state === "not_observed" ? "No lease recorded" : schedulerHistorical ? "Historical lease — refresh needed" : evidence.scheduler.state === "active" ? "Active at sample" : evidence.scheduler.state}</StatusBadge>{evidence.scheduler.owner_id && <><br /><CopyControl value={evidence.scheduler.owner_id} label="Copy scheduler node identity" /><br />Generation {evidence.scheduler.generation}<br />Lease expiry: <Timestamp value={evidence.scheduler.expires_at} /></>}<br />Sampled: <Timestamp value={evidence.scheduler.sampled_at} /></dd></div>
      </dl>
      <p>Observations have separate collection times. Publications show recorded progress. They do not prove that a worker is running now.</p>
    </>}
    {canRead && online && <a className="text-link" href="/api/developer/distributed/evidence" download>Download historical test evidence</a>}
  </section>;
}
