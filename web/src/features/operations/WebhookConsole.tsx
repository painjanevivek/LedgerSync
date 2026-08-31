"use client";

import { WarningCircle } from "@phosphor-icons/react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

import { ConsoleFooter, ConsoleShell } from "@/features/console/ConsoleShell";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { PageHeader, StatePanel } from "@/features/console/components";
import { beginEvidenceRequest, createEvidenceRequestCoordinator, finishEvidenceRequest, invalidateEvidenceRequests, isEvidenceRequestCurrent } from "@/features/console/evidenceRequestCoordinator";
import { WebhookEndpointDetailView, WebhookEndpointListView } from "@/features/operations/WebhookViews";
import type { WebhookEndpoint, WebhookEndpointDetail, WebhookEndpointPage } from "@/lib/api/operations";
import { readJSON, unavailableMessage } from "@/lib/api/client";
import { emptyWebhookFilters, type WebhookFilters, webhooksURL } from "@/lib/page-query/operations";

function endpointQuery(filters: WebhookFilters) {
  const query = new URLSearchParams({ limit: "25" });
  for (const [key, value] of Object.entries(filters)) if (value) query.set(key, value);
  return query.toString();
}

export function WebhookConsole({ endpointId, filters = emptyWebhookFilters, returnTo, invalidQuery = false }: Readonly<{ endpointId?: string; filters?: WebhookFilters; returnTo?: string; invalidQuery?: boolean }>) {
  const router = useRouter();
  const { session, sessionError, sessionLoading, online, hasScope } = useConsoleSession();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [items, setItems] = useState<WebhookEndpoint[]>([]);
  const [endpoint, setEndpoint] = useState<WebhookEndpointDetail | null>(null);
  const [nextCursor, setNextCursor] = useState<string>();
  const [verifiedAt, setVerifiedAt] = useState<string>();
  const listRequests = useRef(createEvidenceRequestCoordinator());
  const detailRequests = useRef(createEvidenceRequestCoordinator());

  const loadList = useCallback(async () => {
    const key = endpointQuery(filters);
    const request = beginEvidenceRequest(listRequests.current, `webhooks:${key}`);
    if (!request) return;
    setLoading(true); setError(null);
    if (!request.sameResource) { setItems([]); setNextCursor(undefined); setVerifiedAt(undefined); }
    const response = await readJSON<WebhookEndpointPage>(`/api/webhook-endpoints?${key}`);
    if (!isEvidenceRequestCurrent(listRequests.current, request.token)) return;
    if (response.ok && Array.isArray(response.data.items)) { setItems(response.data.items); setNextCursor(response.data.next_cursor || undefined); setVerifiedAt(new Date().toISOString()); }
    else setError(unavailableMessage(response.status, "webhook endpoint evidence", response.requestReference));
    if (finishEvidenceRequest(listRequests.current, request.token)) setLoading(false);
  }, [filters]);

  const loadDetail = useCallback(async () => {
    if (!endpointId) return;
    const request = beginEvidenceRequest(detailRequests.current, `webhook:${endpointId}`);
    if (!request) return;
    setLoading(true); setError(null);
    if (!request.sameResource) { setEndpoint(null); setVerifiedAt(undefined); }
    const response = await readJSON<WebhookEndpointDetail>(`/api/webhook-endpoints/${encodeURIComponent(endpointId)}`);
    if (!isEvidenceRequestCurrent(detailRequests.current, request.token)) return;
    if (response.ok && response.data.endpoint_id) { setEndpoint(response.data); setVerifiedAt(new Date().toISOString()); }
    else setError(response.status === 404 ? `The selected webhook endpoint was not found in this authorized tenant scope. Request reference: ${response.requestReference}.` : unavailableMessage(response.status, "webhook endpoint detail", response.requestReference));
    if (finishEvidenceRequest(detailRequests.current, request.token)) setLoading(false);
  }, [endpointId]);

  useEffect(() => {
    if (!session || !online || !hasScope("webhooks:read") || invalidQuery) return;
    const listCoordinator = listRequests.current;
    const detailCoordinator = detailRequests.current;
    const timer = window.setTimeout(() => { void (endpointId ? loadDetail() : loadList()); }, 0);
    return () => { window.clearTimeout(timer); invalidateEvidenceRequests(listCoordinator); invalidateEvidenceRequests(detailCoordinator); };
  }, [endpointId, hasScope, invalidQuery, loadDetail, loadList, online, session]);

  if (sessionLoading) return <ConsoleShell section="events" tenantLabel="Verifying tenant" tenantMeta="Secure session" environmentLabel="Checking environment" operatorLabel="Verifying operator" operatorMeta="Authorization pending"><PageHeader eyebrow="Operations · LedgerSync" title="Verifying access" description="Checking the authorized tenant and webhook read scope before endpoint evidence is displayed."/><StatePanel title="Loading authorized evidence" message="No endpoint or delivery state is being inferred while the session is verified."/><ConsoleFooter/></ConsoleShell>;
  if (!session) return <main className="boot-screen"><p className="eyebrow">Access not verified</p><h1>Operator workspace unavailable</h1><StatePanel kind={sessionError ? "error" : "denied"} title={sessionError ? "Session evidence unavailable" : "No authorized session"} message={sessionError ?? "Log in to the local workspace or configure the approved OIDC provider. No endpoint data is displayed."}/></main>;
  const canRead = hasScope("webhooks:read");
  const canReplay = hasScope("webhooks:replay");
  return <ConsoleShell section="events" tenantLabel={session.tenant_label ?? "Ledger tenant"} tenantMeta={session.tenant_id} environmentLabel={session.environment === "local" ? "Local workspace" : "Verified production"} operatorLabel={session.operator_label ?? session.subject_id} operatorMeta={session.environment === "local" ? "This workstation" : "Authorized operator"}>
    {!online && <div className="offline-banner" role="status"><WarningCircle weight="fill" aria-hidden="true"/><span><strong>You are offline.</strong> Endpoint evidence is retained only with its last verified timestamp.</span></div>}
    {!endpointId && invalidQuery && canRead ? (
      <><PageHeader eyebrow="Operations / Webhooks" title="Webhook endpoints" description="Inspect verification, subscriptions, and bounded delivery health without exposing endpoint paths or signing material."/><StatePanel kind="error" title="Invalid webhook investigation URL" message="The shared URL contains an unknown, repeated, empty, oversized, or malformed filter. No protected webhook request was made." action={<button className="button secondary" type="button" onClick={() => router.replace("/webhooks")}>Clear invalid filters</button>}/></>
    ) : endpointId ? (
      <WebhookEndpointDetailView endpoint={endpoint} tenantId={session.tenant_id} csrfToken={session.csrf_token} verifiedAt={verifiedAt} loading={loading} error={error} online={online} canRead={canRead} canReplay={canReplay} returnTo={returnTo} onRefresh={() => void loadDetail()}/>
    ) : (
      <WebhookEndpointListView key={webhooksURL(filters)} items={items} filters={filters} nextCursor={nextCursor} verifiedAt={verifiedAt} loading={loading} error={error} online={online} canRead={canRead} onRefresh={() => void loadList()}/>
    )}
    <ConsoleFooter/>
  </ConsoleShell>;
}
