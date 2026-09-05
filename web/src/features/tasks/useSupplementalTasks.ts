"use client";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ConsoleSession } from "@/features/accounts/types";
import { deriveConsoleCapabilities } from "@/features/console/capabilities";
import { readJSON } from "@/lib/api/client";
import type { FundingPage } from "@/lib/api/funding";
import type { CorrectionPage } from "@/lib/api/corrections";
import type { EventPage, WebhookEndpointPage } from "@/lib/api/operations";
import type { RecoveryEvidenceIndex } from "@/lib/api/recovery";
import { correctionTasks, deliveryTasks, fundingTasks, recoveryTasks, webhookTasks, type TaskCoverage, type WorkspaceTask } from "./taskPresentation";

type Snapshot = { key: string; coverage: TaskCoverage[]; tasks: Record<string, WorkspaceTask[]> };
export function useSupplementalTasks(session: ConsoleSession | null, online: boolean) {
  const caps = useMemo(() => deriveConsoleCapabilities(session), [session]);
  const key = session ? JSON.stringify([session.tenant_id, session.subject_id, session.scopes]) : "anonymous";
  const sources = useMemo(() => [
    { id: "funding", label: "Money added", href: "/funding", allowed: caps.fundingRead, endpoint: "/api/funding-events?limit=25", adapt: (data: FundingPage) => { if (!Array.isArray(data.events)) throw new Error(); return fundingTasks(data.events); } },
    { id: "corrections", label: "Corrections", href: "/corrections", allowed: caps.correctionsRead, endpoint: "/api/transfer-corrections?limit=25", adapt: (data: CorrectionPage) => { if (!Array.isArray(data.events)) throw new Error(); return correctionTasks(data.events); } },
    { id: "events", label: "Delivery activity", href: "/events", allowed: caps.eventsRead, endpoint: "/api/events?limit=25", adapt: (data: EventPage) => { if (!Array.isArray(data.events)) throw new Error(); return deliveryTasks(data.events); } },
    { id: "webhooks", label: "Delivery destinations", href: "/webhooks", allowed: caps.webhooksRead, endpoint: "/api/webhook-endpoints?limit=25", adapt: (data: WebhookEndpointPage) => { if (!Array.isArray(data.items)) throw new Error(); return webhookTasks(data.items); } },
    { id: "recovery", label: "Recovery setup", href: "/recovery", allowed: caps.recoveryRead, endpoint: "/api/recovery/manifests", adapt: (data: RecoveryEvidenceIndex) => { if (data.format_version !== "ledgersync-recovery-evidence-index/v1") throw new Error(); return recoveryTasks(data); } },
  ], [caps]);
  const initialCoverage = useMemo(() => sources.map(source => ({ id: source.id, label: source.label, href: source.href, state: source.allowed ? "loading" as const : "not-authorized" as const })), [sources]);
  const [snapshot, setSnapshot] = useState<Snapshot>({ key: "", coverage: [], tasks: {} });
  const active = useRef<AbortController | null>(null);
  const refresh = useCallback(async () => {
    if (!session || !online) return;
    active.current?.abort();
    const controller = new AbortController();
    active.current = controller;
    setSnapshot(previous => ({ key, coverage: initialCoverage, tasks: previous.key === key ? previous.tasks : {} }));
    await Promise.all(sources.filter(source => source.allowed).map(async source => {
      const result = await readJSON<FundingPage & CorrectionPage & EventPage & WebhookEndpointPage & RecoveryEvidenceIndex>(source.endpoint, controller.signal);
      if (controller.signal.aborted) return;
      let tasks: WorkspaceTask[] | undefined;
      let state: TaskCoverage["state"] = "unavailable";
      if (result.ok) {
        try { tasks = source.adapt(result.data); state = result.data.next_cursor ? "partial" : "verified"; } catch { /* malformed data never becomes an empty, verified source */ }
      }
      setSnapshot(previous => previous.key !== key ? previous : ({ ...previous, tasks: tasks ? { ...previous.tasks, [source.id]: tasks } : previous.tasks, coverage: previous.coverage.map(item => item.id === source.id ? { ...item, state } : item) }));
    }));
  }, [initialCoverage, key, online, session, sources]);
  useEffect(() => { const timer = window.setTimeout(() => void refresh(), 0); return () => { clearTimeout(timer); active.current?.abort(); }; }, [refresh]);
  const current = snapshot.key === key ? snapshot : { key, coverage: initialCoverage, tasks: {} };
  return { coverage: current.coverage, tasks: Object.values(current.tasks).flat(), refresh };
}
