"use client";

import { WarningCircle } from "@phosphor-icons/react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

import type { ConsoleSession } from "@/features/accounts/types";
import { ConsoleFooter, ConsoleShell } from "@/features/console/ConsoleShell";
import { PageHeader, StatePanel } from "@/features/console/components";
import { EventDetailView, EventsListView, type EventFilters } from "@/features/operations/EventViews";
import { LocalStatusView } from "@/features/operations/LocalStatusView";
import type { DeliveryEvent, DeliveryEventDetail, EventPage, LocalDiagnostics } from "@/lib/api/operations";
import { readJSON, unavailableMessage } from "@/lib/api/client";

type Props = Readonly<{ section: "local-status" | "events"; eventId?: string; filters?: EventFilters; returnTo?: string }>;
const emptyEventFilters: EventFilters = {};

function eventQuery(filters: EventFilters) {
  const query = new URLSearchParams({ limit: "25" });
  for (const [key, value] of Object.entries(filters)) if (value) query.set(key, value);
  return query.toString();
}

export function OperationsConsole({ section, eventId, filters = emptyEventFilters, returnTo }: Props) {
  const router = useRouter();
  const [session, setSession] = useState<ConsoleSession | null>(null);
  const [sessionLoading, setSessionLoading] = useState(true);
  const [online, setOnline] = useState(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [diagnostics, setDiagnostics] = useState<LocalDiagnostics | null>(null);
  const [events, setEvents] = useState<DeliveryEvent[]>([]);
  const [nextCursor, setNextCursor] = useState<string>();
  const [event, setEvent] = useState<DeliveryEventDetail | null>(null);
  const requestGeneration = useRef(0);

  const loadDiagnostics = useCallback(async () => {
    setLoading(true);
    const response = await readJSON<LocalDiagnostics>("/api/local/diagnostics");
    if (response.ok && response.data.overall_state) { setDiagnostics(response.data); setError(null); }
    else setError(unavailableMessage(response.status, "local diagnostics"));
    setLoading(false);
  }, []);

  const loadEvents = useCallback(async () => {
    const generation = ++requestGeneration.current;
    setLoading(true);
    setError(null);
    setEvents([]);
    setNextCursor(undefined);
    const response = await readJSON<EventPage>(`/api/events?${eventQuery(filters)}`);
    if (generation !== requestGeneration.current) return;
    if (response.ok && Array.isArray(response.data.events)) { setEvents(response.data.events); setNextCursor(response.data.next_cursor || undefined); setError(null); }
    else setError(unavailableMessage(response.status, "event evidence"));
    setLoading(false);
  }, [filters]);

  const loadEvent = useCallback(async () => {
    if (!eventId) return;
    const generation = ++requestGeneration.current;
    setLoading(true);
    setError(null);
    setEvent(null);
    const response = await readJSON<DeliveryEventDetail>(`/api/events/${encodeURIComponent(eventId)}`);
    if (generation !== requestGeneration.current) return;
    if (response.ok && response.data.event_id) { setEvent(response.data); setError(null); }
    else setError(response.status === 404 ? "The selected event was not found in this authorized tenant scope." : unavailableMessage(response.status, "event detail"));
    setLoading(false);
  }, [eventId]);

  useEffect(() => {
    let active = true;
    (async () => {
      const response = await readJSON<ConsoleSession>("/api/session");
      if (!active) return;
      if (response.ok && response.data.tenant_id) setSession(response.data);
      setSessionLoading(false);
    })();
    return () => { active = false; };
  }, []);

  useEffect(() => {
    const update = () => setOnline(navigator.onLine);
    update();
    window.addEventListener("online", update);
    window.addEventListener("offline", update);
    return () => { window.removeEventListener("online", update); window.removeEventListener("offline", update); };
  }, []);

  useEffect(() => {
    if (!session || !online) return;
    const timer = window.setTimeout(() => {
      if (section === "local-status" && session.scopes.includes("local:read")) void loadDiagnostics();
      if (section === "events" && session.scopes.includes("events:read")) void (eventId ? loadEvent() : loadEvents());
    }, 0);
    return () => { window.clearTimeout(timer); requestGeneration.current += 1; };
  }, [eventId, loadDiagnostics, loadEvent, loadEvents, online, section, session]);

  async function signOut() {
    if (!session) return;
    await fetch("/api/auth/sign-out", { method: "POST", headers: { "X-CSRF-Token": session.csrf_token } });
    router.refresh();
  }

  if (sessionLoading) return <ConsoleShell section={section} tenantLabel="Verifying tenant" tenantMeta="Secure session" environmentLabel="Checking environment" operatorLabel="Verifying operator" operatorMeta="Authorization pending"><PageHeader eyebrow="Local operations · LedgerSync" title="Verifying access" description="Checking the authorized tenant and read scope before operational evidence is displayed."/><StatePanel title="Loading authorized evidence" message="No dependency or delivery state is being inferred while the session is verified."/><ConsoleFooter/></ConsoleShell>;
  if (!session) return <main className="boot-screen"><p className="eyebrow">Authentication required</p><h1>Operator workspace unavailable</h1><StatePanel kind="denied" title="No authorized session" message="Configure the approved OIDC provider, or explicitly enable the isolated local demo environment. No operational data is displayed." /></main>;

  const canRead = section === "local-status" ? session.scopes.includes("local:read") : session.scopes.includes("events:read");
  return <ConsoleShell section={section} tenantLabel={session.tenant_label ?? "Ledger tenant"} tenantMeta={session.tenant_id} environmentLabel={session.environment === "demo" ? "Isolated demo" : "Verified production"} operatorLabel={session.operator_label ?? session.subject_id} operatorMeta={session.environment === "demo" ? "Non-production data" : "Authorized operator"} preview={session.environment === "demo"} onSignOut={() => void signOut()}>
    {!online && <div className="offline-banner" role="status"><WarningCircle weight="fill" aria-hidden="true"/><span><strong>You are offline.</strong> Read evidence is retained only with its last verified timestamp.</span></div>}
    {section === "local-status" && <LocalStatusView evidence={diagnostics} loading={loading} error={error} online={online} canRead={canRead} onRefresh={() => void loadDiagnostics()} />}
    {section === "events" && eventId && <EventDetailView event={event} loading={loading} error={error} online={online} canRead={canRead} returnTo={returnTo} onRefresh={() => void loadEvent()} />}
    {section === "events" && !eventId && <EventsListView events={events} filters={filters} nextCursor={nextCursor} loading={loading} error={error} online={online} canRead={canRead} onRefresh={() => void loadEvents()} />}
    <ConsoleFooter/>
  </ConsoleShell>;
}
