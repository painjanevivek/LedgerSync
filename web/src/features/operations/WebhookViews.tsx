"use client";

import { ArrowClockwise, ArrowLeft, CheckCircle, WarningCircle } from "@phosphor-icons/react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { CopyControl } from "@/ui/controls/CopyControl.client";
import { DataTableRegion } from "@/ui/display/DataTableRegion";
import { EvidenceFreshness } from "@/ui/display/Evidence";
import { SavedViewCapture } from "@/features/investigation/SavedViewCapture";
import { PageHeader } from "@/ui/display/PageHeader";
import { RecordLink } from "@/ui/display/RecordLink";
import { StatePanel } from "@/ui/display/StatePanel";
import { StatusBadge } from "@/ui/display/StatusBadge";
import { FormField } from "@/ui/forms/FormField.client";
import { WebhookReplayControl } from "@/features/operations/WebhookReplayControl";
import type { WebhookDeliveryAttempt, WebhookEndpoint, WebhookEndpointDetail } from "@/lib/api/operations";
import { webhooksURL, type WebhookFilters } from "@/lib/page-query/operations";
import { AdvancedFilterPanel } from "@/ui/disclosure/AdvancedFilterPanel";
import { DisclosureSection } from "@/ui/disclosure/DisclosureSection";
import { NextBestAction } from "@/ui/disclosure/NextBestAction";

function utc(value?: string) {
  if (!value) return "Not available";
  const date = new Date(value);
  return Number.isNaN(date.valueOf()) ? "Not available" : date.toISOString().replace("T", " ").replace(".000Z", " UTC");
}

function label(value: string) { return value.replaceAll("_", " ").replace(/^./, (character) => character.toUpperCase()); }
function endpointTone(status: WebhookEndpoint["status"]) { return status === "active" ? "success" as const : status === "disabled" ? "danger" as const : "warning" as const; }
function deliveryTone(state: WebhookEndpoint["recent_delivery_state"] | WebhookDeliveryAttempt["state"]) { return state === "delivered" || state === "none" ? "success" as const : state === "dead" ? "danger" as const : "warning" as const; }

export function WebhookEndpointListView({ items, filters, nextCursor, verifiedAt, loading, error, online, canRead, onRefresh }: Readonly<{ items: WebhookEndpoint[]; filters: WebhookFilters; nextCursor?: string; verifiedAt?: string; loading: boolean; error: string | null; online: boolean; canRead: boolean; onRefresh: () => void }>) {
  const router = useRouter();
  const returnTo = webhooksURL(filters);
  const nextHref = nextCursor ? webhooksURL({ ...filters, cursor: nextCursor }) : undefined;
  const attentionCount = items.filter((item) => item.status !== "active" || item.recent_delivery_state === "dead").length;
  return <>
    <PageHeader eyebrow="Operations / Webhooks" title="Webhook endpoints" description="Inspect verification, subscriptions, and bounded delivery health without exposing endpoint paths or signing material."><button className="button secondary guarded-control" type="button" disabled={!online || loading || !canRead} onClick={onRefresh}><ArrowClockwise aria-hidden="true"/>Refresh endpoints</button></PageHeader>
    <nav className="related-evidence" aria-label="Delivery evidence views"><Link className="record-link" href="/events">Events</Link><Link className="record-link" aria-current="page" href="/webhooks">Webhook endpoints</Link></nav>
    <div className="financial-separation-note"><CheckCircle weight="fill" aria-hidden="true"/><div><strong>Delivery controls do not post money</strong><p>A dead or disabled webhook does not roll back a committed transfer. Open the linked event and transfer records to verify each independent outcome.</p></div></div>
    {!canRead && <StatePanel kind="denied" title="Webhook evidence not authorized" message="This session does not include webhooks:read. No endpoint metadata has been requested."/>}
    {!online && <StatePanel kind="offline" title="Offline — endpoint evidence is historical" message="Reconnect before refreshing or treating delivery health as current."/>}
    {attentionCount > 0 && <NextBestAction attention eyebrow="Endpoint health" title={`${attentionCount} endpoint${attentionCount === 1 ? "" : "s"} need verification or delivery review`} message="Open the affected endpoint before reviewing healthy destination history." />}
    <AdvancedFilterPanel id="webhook-endpoint-filters" title="Filter endpoints" activeCount={Number(Boolean(filters.status)) + Number(Boolean(filters.eventType))}>
    <form className="event-filter-document" aria-label="Webhook endpoint filters" onSubmit={(submitEvent) => {
      submitEvent.preventDefault();
      const data = new FormData(submitEvent.currentTarget);
      const status = String(data.get("status") ?? "").trim();
      const eventType = String(data.get("eventType") ?? "").trim();
      router.push(webhooksURL({ status: (status || undefined) as WebhookFilters["status"], eventType: eventType || undefined }));
    }}>
      <FormField label="Endpoint status" requirement="optional"><select name="status" defaultValue={filters.status ?? ""}><option value="">All statuses</option><option value="pending_verification">Pending verification</option><option value="active">Active</option><option value="disabled">Disabled</option></select></FormField>
      <FormField label="Subscribed event" requirement="optional" hint="Example: transfer.posted"><input name="eventType" defaultValue={filters.eventType} maxLength={128}/></FormField>
      <div className="event-filter-actions"><button className="button primary guarded-control" type="submit">Apply filters</button><Link className="button secondary guarded-control" href="/webhooks">Clear filters</Link></div>
    </form>
    <SavedViewCapture domain="webhooks" filters={{ status: filters.status, eventType: filters.eventType }} />
    </AdvancedFilterPanel>
    {error && <StatePanel kind="error" title="Webhook evidence unavailable" message={error}/>}
    {verifiedAt && items.length > 0 && <EvidenceFreshness state={error || !online ? "historical" : loading ? "refreshing" : "current"} verifiedAt={verifiedAt} label="Endpoint page" reason={error ?? (!online ? "Reconnect before relying on delivery health." : undefined)}/>}
    {loading && items.length === 0 ? <StatePanel title="Loading endpoint evidence" message="Requesting the current bounded endpoint page. No delivery result is being inferred."/> : canRead && !error && items.length === 0 ? <StatePanel title="No endpoints match these filters" message="Change or clear the filters. A truly empty page means no endpoint metadata exists in this authorized tenant scope."/> : items.length > 0 && <section className="ledger-section" aria-labelledby="webhook-list-heading" aria-busy={loading}><div className="section-heading"><div><p className="eyebrow">Most recently updated first</p><h2 id="webhook-list-heading">Registered destinations</h2><p>{items.length} endpoint{items.length === 1 ? "" : "s"} on this page. A total is not calculated or implied.</p></div></div><DataTableRegion label="Webhook endpoint records"><table className="data-table"><thead><tr><th scope="col">Endpoint</th><th scope="col">Status</th><th scope="col">Recent delivery</th><th scope="col">Subscriptions</th><th scope="col">Updated</th><th scope="col">Evidence</th></tr></thead><tbody>{items.map((item) => <tr key={item.endpoint_id}><td><strong>{item.label}</strong><br/><code>{item.origin}</code><CopyControl value={item.endpoint_id} label={`Copy endpoint ${item.label} ID`}/></td><td><StatusBadge tone={endpointTone(item.status)}>{label(item.status)}</StatusBadge></td><td><StatusBadge tone={deliveryTone(item.recent_delivery_state)}>{label(item.recent_delivery_state)}</StatusBadge><br/><span>{item.recent_attempt_count} recent · {item.recent_dead_count} dead</span></td><td>{item.subscribed_events.map((eventType) => <code key={eventType}>{eventType}<br/></code>)}</td><td>{utc(item.updated_at)}</td><td><RecordLink href={`/webhooks/${encodeURIComponent(item.endpoint_id)}?return_to=${encodeURIComponent(returnTo)}`} label="Open endpoint"/></td></tr>)}</tbody></table></DataTableRegion><div className="pagination"><span>{nextHref ? "More endpoint evidence is available" : "End of available endpoints"}</span>{nextHref ? <Link className="button secondary guarded-control" href={nextHref}>Next page</Link> : <button className="button secondary guarded-control" type="button" disabled>Next page</button>}</div></section>}
  </>;
}

export function WebhookEndpointDetailView({ endpoint, tenantId, csrfToken, verifiedAt, loading, error, online, canRead, canReplay, returnTo, onRefresh }: Readonly<{ endpoint: WebhookEndpointDetail | null; tenantId: string; csrfToken: string; verifiedAt?: string; loading: boolean; error: string | null; online: boolean; canRead: boolean; canReplay: boolean; returnTo?: string; onRefresh: () => void }>) {
  return <>
    <Link className="record-link back-link" href={returnTo ?? "/webhooks"}><ArrowLeft aria-hidden="true"/>Back to previous view</Link>
    <PageHeader eyebrow="Webhook evidence / Read-only detail" title={endpoint?.label ?? "Endpoint detail"} description="Inspect safe endpoint metadata and the newest bounded delivery attempts."><button className="button secondary guarded-control" type="button" disabled={!online || loading || !canRead} onClick={onRefresh}><ArrowClockwise aria-hidden="true"/>Refresh endpoint</button></PageHeader>
    {!canRead && <StatePanel kind="denied" title="Webhook evidence not authorized" message="This session does not include webhooks:read. No endpoint detail has been requested."/>}
    {!online && <StatePanel kind="offline" title="Offline — detail is not current" message="The last verified endpoint detail may remain visible. Reconnect before acting on it."/>}
    {error && <StatePanel kind="error" title="Endpoint detail unavailable" message={error}/>}
    {verifiedAt && endpoint && <EvidenceFreshness state={error || !online ? "historical" : loading ? "refreshing" : "current"} verifiedAt={verifiedAt} label="Endpoint detail" reason={error ?? (!online ? "Reconnect before relying on delivery health." : undefined)}/>}
    {loading && !endpoint && <StatePanel title="Loading endpoint detail" message="Requesting authorized endpoint evidence. No delivery or financial status is being inferred."/>}
    {endpoint && <article className="event-detail-document" aria-labelledby="webhook-detail-heading" aria-busy={loading}>
      <header><div><p className="eyebrow">Safe destination evidence</p><h2 id="webhook-detail-heading">{endpoint.label}</h2><CopyControl value={endpoint.endpoint_id} label="Copy endpoint ID"/></div><StatusBadge tone={endpointTone(endpoint.status)}>{label(endpoint.status)}</StatusBadge></header>
      {endpoint.recent_delivery_state === "dead" && <div className="delivery-impact-statement dead"><WarningCircle weight="fill" aria-hidden="true"/><div><strong>Delivery stopped; the financial record is unchanged</strong><p>Review the event and transfer evidence below. A replay resends the existing event only and requires an independently approved command.</p></div></div>}
      {endpoint.status === "pending_verification" && <NextBestAction attention title="Verify this endpoint before expecting delivery" message="The destination is registered but not active. Complete the existing out-of-band verification process; LedgerSync does not expose signing material here." />}
      <dl className="evidence-list"><div><dt>Safe origin</dt><dd><code>{endpoint.origin}</code></dd></div><div><dt>Recent delivery state</dt><dd><StatusBadge tone={deliveryTone(endpoint.recent_delivery_state)}>{label(endpoint.recent_delivery_state)}</StatusBadge></dd></div><div><dt>Recent bounded attempts</dt><dd>{endpoint.recent_attempt_count}</dd></div><div><dt>Recent dead attempts</dt><dd>{endpoint.recent_dead_count}</dd></div><div><dt>Verified</dt><dd>{utc(endpoint.verified_at)}</dd></div><div><dt>Disabled</dt><dd>{utc(endpoint.disabled_at)}</dd></div><div><dt>Updated</dt><dd>{utc(endpoint.updated_at)}</dd></div></dl>
      <DisclosureSection id="webhook-subscriptions" title="Subscriptions and signing-key metadata" summary={`${endpoint.subscribed_events.length} declared event subscription${endpoint.subscribed_events.length === 1 ? "" : "s"}`} lazy><section className="ledger-section" aria-labelledby="webhook-subscriptions-heading"><div className="section-heading"><div><p className="eyebrow">Declared metadata</p><h3 id="webhook-subscriptions-heading">Subscriptions</h3></div></div><p>{endpoint.subscribed_events.map((eventType) => <code key={eventType}>{eventType} </code>)}</p></section></DisclosureSection>
      <DisclosureSection id="webhook-delivery-attempts" title="Delivery attempts" summary={`${endpoint.delivery_attempts.length} bounded immutable attempt${endpoint.delivery_attempts.length === 1 ? "" : "s"}`} attention={endpoint.recent_delivery_state === "dead"} lazy><section className="ledger-section" aria-labelledby="webhook-attempts-heading"><div className="section-heading"><div><p className="eyebrow">Newest immutable evidence</p><h3 id="webhook-attempts-heading">Delivery attempts</h3></div></div>{endpoint.delivery_attempts_truncated && <StatePanel kind="unknown" title="Older attempts are not shown" message="This detail is intentionally bounded to the newest 25 attempts."/>}{endpoint.delivery_attempts.length === 0 ? <StatePanel title="No delivery attempts recorded" message="The endpoint exists, but no immutable attempt evidence is available yet."/> : <DataTableRegion label="Webhook delivery attempts"><table className="data-table"><thead><tr><th scope="col">Attempt</th><th scope="col">State</th><th scope="col">Result</th><th scope="col">Completed</th><th scope="col">Related evidence</th></tr></thead><tbody>{endpoint.delivery_attempts.map((attempt) => <tr key={attempt.attempt_id}><td><strong>Attempt {attempt.attempt_number}</strong><CopyControl value={attempt.attempt_id} label={`Copy attempt ${attempt.attempt_number} ID`}/></td><td><StatusBadge tone={deliveryTone(attempt.state)}>{label(attempt.state)}</StatusBadge></td><td><code>{attempt.response_class ?? attempt.error_code ?? "Not available"}</code></td><td>{utc(attempt.completed_at)}</td><td><nav className="event-related-links" aria-label={`Related evidence for attempt ${attempt.attempt_number}`}>{attempt.event_id && <RecordLink href={`/events/${encodeURIComponent(attempt.event_id)}?return_to=${encodeURIComponent(`/webhooks/${endpoint.endpoint_id}`)}`} label="Event"/>}<RecordLink href={`/transfers/${encodeURIComponent(attempt.transfer_id)}?return_to=${encodeURIComponent(`/webhooks/${endpoint.endpoint_id}`)}`} label="Transfer"/></nav></td></tr>)}</tbody></table></DataTableRegion>}</section></DisclosureSection>
      {endpoint.delivery_attempts.filter((attempt) => attempt.state === "dead").map((attempt) => <WebhookReplayControl key={attempt.attempt_id} tenantId={tenantId} endpointId={endpoint.endpoint_id} attemptId={attempt.attempt_id} csrfToken={csrfToken} online={online} canReplay={canReplay} onScheduled={onRefresh}/>)}
    </article>}
  </>;
}
