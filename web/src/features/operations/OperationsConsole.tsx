"use client";

import { WarningCircle } from "@phosphor-icons/react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

import { ConsoleFooter, ConsoleShell } from "@/features/console/ConsoleShell";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { PageHeader, StatePanel } from "@/features/console/components";
import { beginEvidenceRequest, createEvidenceRequestCoordinator, finishEvidenceRequest, invalidateEvidenceRequests, isEvidenceRequestCurrent } from "@/features/console/evidenceRequestCoordinator";
import { EventDetailView, EventsListView } from "@/features/operations/EventViews";
import { LocalStatusView } from "@/features/operations/LocalStatusView";
import type { DeliveryEvent, DeliveryEventDetail, EventPage, LocalDiagnostics } from "@/lib/api/operations";
import { readJSON, unavailableMessage } from "@/lib/api/client";
import { emptyEventFilters, eventsURL, type EventFilters } from "@/lib/page-query/operations";

type Props = Readonly<{ section: "local-status" | "events"; eventId?: string; filters?: EventFilters; returnTo?: string; invalidQuery?: boolean }>;

function eventQuery(filters: EventFilters) {
  const query = new URLSearchParams({ limit: "25" });
  for (const [key, value] of Object.entries(filters)) if (value) query.set(key, value);
  return query.toString();
}

export function OperationsConsole({ section, eventId, filters = emptyEventFilters, returnTo, invalidQuery = false }: Props) {
  const router = useRouter();
  const { session, sessionError, sessionLoading, online, hasScope } = useConsoleSession();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [diagnostics, setDiagnostics] = useState<LocalDiagnostics | null>(null);
  const [events, setEvents] = useState<DeliveryEvent[]>([]);
  const [nextCursor, setNextCursor] = useState<string>();
  const [event, setEvent] = useState<DeliveryEventDetail | null>(null);
  const [verifiedAt, setVerifiedAt] = useState<string>();
  const diagnosticsRequests = useRef(createEvidenceRequestCoordinator());
  const eventListRequests = useRef(createEvidenceRequestCoordinator());
  const eventDetailRequests = useRef(createEvidenceRequestCoordinator());

  const loadDiagnostics = useCallback(async () => {
    const request = beginEvidenceRequest(diagnosticsRequests.current, "local-diagnostics");
    if (!request) return;
    setLoading(true);
    const response = await readJSON<LocalDiagnostics>("/api/local/diagnostics");
    if (!isEvidenceRequestCurrent(diagnosticsRequests.current, request.token)) return;
    if (response.ok && response.data.overall_state) { setDiagnostics(response.data); setVerifiedAt(response.data.generated_at); setError(null); }
    else setError(unavailableMessage(response.status, "local diagnostics", response.requestReference));
    if (finishEvidenceRequest(diagnosticsRequests.current, request.token)) setLoading(false);
  }, []);

  const loadEvents = useCallback(async () => {
    const key = eventQuery(filters);
    const request = beginEvidenceRequest(eventListRequests.current, `events:${key}`);
    if (!request) return;
    setLoading(true);
    setError(null);
    if (!request.sameResource) { setEvents([]); setNextCursor(undefined); setVerifiedAt(undefined); }
    const response = await readJSON<EventPage>(`/api/events?${key}`);
    if (!isEvidenceRequestCurrent(eventListRequests.current, request.token)) return;
    if (response.ok && Array.isArray(response.data.events)) { setEvents(response.data.events); setNextCursor(response.data.next_cursor || undefined); setVerifiedAt(new Date().toISOString()); setError(null); }
    else setError(unavailableMessage(response.status, "event evidence", response.requestReference));
    if (finishEvidenceRequest(eventListRequests.current, request.token)) setLoading(false);
  }, [filters]);

  const loadEvent = useCallback(async () => {
    if (!eventId) return;
    const key = `event:${eventId}`;
    const request = beginEvidenceRequest(eventDetailRequests.current, key);
    if (!request) return;
    setLoading(true);
    setError(null);
    if (!request.sameResource) { setEvent(null); setVerifiedAt(undefined); }
    const response = await readJSON<DeliveryEventDetail>(`/api/events/${encodeURIComponent(eventId)}`);
    if (!isEvidenceRequestCurrent(eventDetailRequests.current, request.token)) return;
    if (response.ok && response.data.event_id) { setEvent(response.data); setVerifiedAt(new Date().toISOString()); setError(null); }
    else setError(response.status === 404 ? `The selected event was not found in this authorized tenant scope. Request reference: ${response.requestReference}.` : unavailableMessage(response.status, "event detail", response.requestReference));
    if (finishEvidenceRequest(eventDetailRequests.current, request.token)) setLoading(false);
  }, [eventId]);

  useEffect(() => {
    if (!session || !online) return;
    const diagnosticsCoordinator = diagnosticsRequests.current;
    const eventListCoordinator = eventListRequests.current;
    const eventDetailCoordinator = eventDetailRequests.current;
    const timer = window.setTimeout(() => {
      if (section === "local-status" && session.environment === "local" && hasScope("local:read")) void loadDiagnostics();
      if (section === "events" && hasScope("events:read") && !invalidQuery) void (eventId ? loadEvent() : loadEvents());
    }, 0);
    return () => { window.clearTimeout(timer); invalidateEvidenceRequests(diagnosticsCoordinator); invalidateEvidenceRequests(eventListCoordinator); invalidateEvidenceRequests(eventDetailCoordinator); };
  }, [eventId, hasScope, invalidQuery, loadDiagnostics, loadEvent, loadEvents, online, section, session]);

  if (sessionLoading) return <ConsoleShell section={section} tenantLabel="Verifying tenant" tenantMeta="Secure session" environmentLabel="Checking environment" operatorLabel="Verifying operator" operatorMeta="Authorization pending"><PageHeader eyebrow="Local operations · LedgerSync" title="Verifying access" description="Checking the authorized tenant and read scope before operational evidence is displayed."/><StatePanel title="Loading authorized evidence" message="No dependency or delivery state is being inferred while the session is verified."/><ConsoleFooter/></ConsoleShell>;
  if (!session) return <main className="boot-screen"><p className="eyebrow">Access not verified</p><h1>Operator workspace unavailable</h1><StatePanel kind={sessionError ? "error" : "denied"} title={sessionError ? "Session evidence unavailable" : "No authorized session"} message={sessionError ?? "Log in to the local workspace or configure the approved OIDC provider. No operational data is displayed."} /></main>;

  const canRead = section === "local-status" ? session.environment === "local" && hasScope("local:read") : hasScope("events:read");
  return <ConsoleShell section={section} tenantLabel={session.tenant_label ?? "Ledger tenant"} tenantMeta={session.tenant_id} environmentLabel={session.environment === "local" ? "Local workspace" : "Verified production"} operatorLabel={session.operator_label ?? session.subject_id} operatorMeta={session.environment === "local" ? "This workstation" : "Authorized operator"}>
    {!online && <div className="offline-banner" role="status"><WarningCircle weight="fill" aria-hidden="true"/><span><strong>You are offline.</strong> Read evidence is retained only with its last verified timestamp.</span></div>}
    {section === "local-status" && <LocalStatusView evidence={diagnostics} verifiedAt={verifiedAt} loading={loading} error={error} online={online} canRead={canRead} onRefresh={() => void loadDiagnostics()} />}
    {section === "events" && eventId && <EventDetailView event={event} verifiedAt={verifiedAt} loading={loading} error={error} online={online} canRead={canRead} returnTo={returnTo} onRefresh={() => void loadEvent()} />}
    {section === "events" && !eventId && invalidQuery && canRead ? <><PageHeader eyebrow="Operations / Delivery" title="Delivery events" description="Check whether LedgerSync sent each update. This page does not change money or resend an event."/><StatePanel kind="error" title="Invalid event investigation URL" message="The shared URL contains an unknown, repeated, empty, oversized, malformed, or reversed filter. No protected event request was made." action={<button className="button secondary" type="button" onClick={() => router.replace("/events")}>Clear invalid filters</button>}/></> : section === "events" && !eventId && <EventsListView key={eventsURL(filters)} events={events} filters={filters} nextCursor={nextCursor} verifiedAt={verifiedAt} loading={loading} error={error} online={online} canRead={canRead} onRefresh={() => void loadEvents()} />}
    <ConsoleFooter/>
  </ConsoleShell>;
}
