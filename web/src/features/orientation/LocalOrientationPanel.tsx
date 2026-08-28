"use client";

import { CheckCircle, Compass, Info, WarningCircle, X } from "@phosphor-icons/react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { CopyControl, EvidenceFreshness, StatusBadge } from "@/features/console/components";
import { utcDateTime } from "@/features/console/format";
import type { LocalOrientation, OrientationStep } from "@/lib/api/orientation";

const stepCopy: Record<OrientationStep["id"], { title: string; description: string; href: string }> = {
  inspect_account: { title: "Inspect an account", description: "Read its exact INR balance and immutable history.", href: "/accounts" },
  create_account: { title: "Create an account", description: "Create an active INR account with an exact zero boundary.", href: "/accounts/new?return_to=%2Faccounts" },
  fund_account: { title: "Fund through a transfer", description: "Move value internally from another authorized account.", href: "/transfers" },
  inspect_transfer: { title: "Inspect a transfer", description: "Open the posted record and its double-entry proof.", href: "/transfers" },
  run_reconciliation: { title: "Run reconciliation", description: "Compare ledger postings with PostgreSQL balance truth.", href: "/reconciliation" },
  inspect_delivery: { title: "Inspect delivery", description: "Review outbox and downstream attempts separately from money.", href: "/events" },
  create_backup: { title: "Create a protected backup", description: "Run the fixed host command, then inspect recovery evidence.", href: "/recovery" },
};

function evidenceHref(step: OrientationStep) {
  if (!step.evidence_id) return stepCopy[step.id].href;
  if (step.id === "inspect_account") return `/accounts/${encodeURIComponent(step.evidence_id)}`;
  if (step.id === "fund_account" || step.id === "inspect_transfer") return `/transfers/${encodeURIComponent(step.evidence_id)}`;
  if (step.id === "run_reconciliation") return `/reconciliation/${encodeURIComponent(step.evidence_id)}`;
  if (step.id === "inspect_delivery") return `/events/${encodeURIComponent(step.evidence_id)}`;
  return stepCopy[step.id].href;
}

function tone(step: OrientationStep) { return step.state === "completed" ? "success" as const : step.state === "evidence_available" ? "info" as const : "warning" as const; }
function stateLabel(step: OrientationStep) { return step.state === "completed" ? "Completed" : step.state === "evidence_available" ? "Evidence available" : step.state === "unavailable" ? "Evidence unavailable" : "Not yet evidenced"; }

type Props = Readonly<{ tenantId: string; evidence: LocalOrientation | null; loading: boolean; error: string | null; online: boolean; canRead: boolean; forceOpen?: boolean; onRefresh: () => void }>;

export function LocalOrientationPanel({ tenantId, evidence, loading, error, online, canRead, forceOpen = false, onRefresh }: Props) {
  const storageKey = `ledgersync:orientation-dismissed:${tenantId}`;
  const [visible, setVisible] = useState(forceOpen);
  useEffect(() => {
    const timer=window.setTimeout(()=>{ if (forceOpen) { setVisible(true); return; } try { setVisible(localStorage.getItem(storageKey) !== "true"); } catch { setVisible(true); } },0);
    return ()=>window.clearTimeout(timer);
  }, [forceOpen, storageKey]);
  function dismiss() { try { localStorage.setItem(storageKey, "true"); } catch { /* Dismissal is convenience state only. */ } setVisible(false); }
  if (!visible) return null;

  return <section className="local-orientation" aria-labelledby="local-orientation-title">
    <header>
      <Compass weight="fill" aria-hidden="true" />
      <div><p className="eyebrow">Local guide / Safe demonstration</p><h2 id="local-orientation-title">Follow one INR ledger record from intent to evidence</h2><p>This workspace uses a local demo operator. Transfers are internal-only, and PostgreSQL—not the browser or Redis—remains the financial authority.</p></div>
      <button className="orientation-dismiss" type="button" onClick={dismiss} aria-label="Dismiss local guide"><X aria-hidden="true" /></button>
    </header>
    <div className="orientation-boundary" aria-label="Local demo boundaries">
      <div><strong>Currency</strong><span>INR only</span></div>
      <div><strong>Movement</strong><span>Authorized internal accounts</span></div>
      <div><strong>Authority</strong><span>PostgreSQL ledger</span></div>
      <div><strong>Persistence</strong><span>Data survives a safe stop</span></div>
    </div>
    {!canRead ? <div className="orientation-state"><WarningCircle weight="fill" aria-hidden="true"/><p><strong>Checklist permission required</strong><span>The guide remains readable, but durable progress needs the local:read scope.</span></p></div>
      : !evidence ? !online ? <div className="orientation-state"><WarningCircle weight="fill" aria-hidden="true"/><p><strong>Checklist unavailable offline</strong><span>No locally cached completion is presented as durable evidence.</span></p></div>
      : error ? <div className="orientation-state" role="alert"><WarningCircle weight="fill" aria-hidden="true"/><p><strong>Durable checklist unavailable</strong><span>{error}</span></p><button className="button secondary" type="button" onClick={onRefresh}>Retry evidence</button></div>
      : <div className="orientation-state" aria-busy="true"><Info weight="fill" aria-hidden="true"/><p><strong>Loading durable progress</strong><span>LedgerSync is checking stored tenant evidence; browser history is not used.</span></p></div>
      : <><EvidenceFreshness state={error || !online ? "historical" : loading ? "refreshing" : "current"} verifiedAt={evidence.generated_at} label="Orientation checklist" reason={error ?? (!online ? "Reconnect before relying on checklist state." : undefined)} /><ol className="orientation-checklist">
        {evidence.steps.map((step) => <li key={step.id}>
          <span className={`orientation-check ${step.state}`} aria-hidden="true">{step.state === "completed" ? <CheckCircle weight="fill" /> : step.state === "evidence_available" ? <Info weight="fill" /> : <WarningCircle weight="fill" />}</span>
          <div><strong>{stepCopy[step.id].title}</strong><p>{stepCopy[step.id].description}</p>{step.occurred_at && <small>Stored evidence · {utcDateTime(step.occurred_at)}</small>}</div>
          <StatusBadge tone={tone(step)}>{stateLabel(step)}</StatusBadge>
          <Link className="record-link" href={evidenceHref(step)}>{step.evidence_id ? "Open evidence" : "Open step"}</Link>
        </li>)}
      </ol></>}
    <footer><div><strong>Stop safely without deleting persisted data</strong><p>Run the fixed host command outside the browser. Reset and reseed controls are intentionally absent.</p></div><CopyControl value="powershell -File .\scripts\stop-local.ps1" label="Copy safe stop command" /></footer>
  </section>;
}
