"use client";

import { ArrowClockwise, ArrowLeft, CheckCircle, Clock, WarningCircle } from "@phosphor-icons/react";
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
import type { DeliveryEvent, DeliveryEventDetail } from "@/lib/api/operations";
import { eventsURL, type EventFilters } from "@/lib/page-query/operations";
import { RelatedEvidenceRail } from "@/features/investigation/RelatedEvidenceRail";

function utc(value?: string) {
  if (!value) return "Not available";
  const date = new Date(value);
  return Number.isNaN(date.valueOf()) ? "Not available" : date.toISOString().replace("T", " ").replace(".000Z", " UTC");
}
function label(value: string) { return value.replaceAll("_", " ").replace(/^./, (character) => character.toUpperCase()); }
function stateTone(state: DeliveryEvent["state"]) { return state === "published" ? "success" as const : state === "dead" ? "danger" as const : "warning" as const; }

export function EventsListView({ events, filters, nextCursor, verifiedAt, loading, error, online, canRead, onRefresh }: Readonly<{ events: DeliveryEvent[]; filters: EventFilters; nextCursor?: string; verifiedAt?: string; loading: boolean; error: string | null; online: boolean; canRead: boolean; onRefresh: () => void }>) {
  const router = useRouter();
  const returnTo = eventsURL(filters);
  const nextHref = nextCursor ? eventsURL({ ...filters, cursor: nextCursor }) : undefined;
  return <>
    <PageHeader eyebrow="Operations / Delivery" title="Delivery events" description="Check whether LedgerSync sent each update. This page does not change money or resend an event."><button className="button secondary guarded-control" type="button" disabled={!online || loading || !canRead} onClick={onRefresh}><ArrowClockwise aria-hidden="true" />Refresh events</button></PageHeader>
    <nav className="related-evidence" aria-label="Delivery evidence views"><Link className="record-link" aria-current="page" href="/events">Events</Link><Link className="record-link" href="/webhooks">Webhook endpoints</Link></nav>
    <div className="financial-separation-note"><CheckCircle weight="fill" aria-hidden="true"/><div><strong>Financial truth remains separate</strong><p>An event can be pending, retrying, or dead after its PostgreSQL transaction has committed. Verify any money result through its linked transfer or account evidence.</p></div></div>
    {!canRead && <StatePanel kind="denied" title="Event evidence not authorized" message="This session does not include events:read. No event records have been requested." />}
    {!online && <StatePanel kind="offline" title="Offline — event evidence is not current" message="Reconnect before refreshing or treating this list as current." />}
    <form className="event-filter-document" aria-label="Event filters" onSubmit={(submitEvent) => {
      submitEvent.preventDefault();
      const data = new FormData(submitEvent.currentTarget);
      const value = (name: string) => String(data.get(name) ?? "").trim();
      router.push(eventsURL({
        eventType: value("eventType") || undefined,
        state: (value("state") || undefined) as EventFilters["state"],
        endpointId: value("endpointId") || undefined,
        relatedId: value("relatedId") || undefined,
        correlationId: value("correlationId") || undefined,
        from: value("from") || undefined,
        to: value("to") || undefined,
      }));
    }}>
      <FormField label="Event type" requirement="optional"><input name="eventType" defaultValue={filters.eventType} maxLength={256} /></FormField>
      <FormField label="State" requirement="optional"><select name="state" defaultValue={filters.state ?? ""}><option value="">All states</option><option value="pending">Pending</option><option value="retrying">Retrying</option><option value="published">Published</option><option value="dead">Dead</option></select></FormField>
      <FormField label="Webhook endpoint ID" requirement="optional" hint="Limit events to one registered endpoint."><input name="endpointId" defaultValue={filters.endpointId} maxLength={36} /></FormField>
      <FormField label="Related ID" requirement="optional" hint="Use an account or transfer ID."><input name="relatedId" defaultValue={filters.relatedId} maxLength={36} /></FormField>
      <FormField label="Correlation ID" requirement="optional" hint="Use the ID that links related activity."><input name="correlationId" defaultValue={filters.correlationId} maxLength={36} /></FormField>
      <FormField label="From UTC" requirement="optional" hint="Example: 2026-08-25T00:00:00Z."><input name="from" defaultValue={filters.from} placeholder="2026-08-25T00:00:00Z" maxLength={64} /></FormField>
      <FormField label="To UTC" requirement="optional" hint="Example: 2026-08-25T23:59:59Z."><input name="to" defaultValue={filters.to} placeholder="2026-08-25T23:59:59Z" maxLength={64} /></FormField>
      <div className="event-filter-actions"><button className="button primary guarded-control" type="submit">Apply filters</button><Link className="button secondary guarded-control" href="/events">Clear filters</Link></div>
    </form>
    <SavedViewCapture domain="events" filters={{ eventType: filters.eventType, state: filters.state, endpointId: filters.endpointId, relatedId: filters.relatedId, correlationId: filters.correlationId, from: filters.from, to: filters.to }} />
    {error && <StatePanel kind="error" title="Event evidence unavailable" message={error} />}
    {verifiedAt && events.length > 0 && <EvidenceFreshness state={error || !online ? "historical" : loading ? "refreshing" : "current"} verifiedAt={verifiedAt} label="Event page" reason={error ?? (!online ? "Reconnect before relying on delivery state." : undefined)} />}
    {loading && events.length === 0 ? <StatePanel title="Loading event evidence" message="Requesting the current bounded event page. No delivery result is being inferred." /> : canRead && !error && events.length === 0 ? <StatePanel title="No events match these filters" message="Change or clear the filters to inspect another bounded tenant-scoped page." /> : events.length > 0 && <section className="ledger-section" aria-labelledby="event-list-heading" aria-busy={loading}><div className="section-heading"><div><p className="eyebrow">Newest occurrence first</p><h2 id="event-list-heading">Delivery records</h2><p>{events.length} event{events.length === 1 ? "" : "s"} on this page. A total is not calculated or implied.</p></div></div><DataTableRegion label="Delivery event records"><table className="data-table event-table"><thead><tr><th scope="col">Event</th><th scope="col">State</th><th scope="col">Related record</th><th scope="col">Attempts</th><th scope="col">Occurred</th><th scope="col">Evidence</th></tr></thead><tbody>{events.map((event) => <tr key={event.event_id}><td><strong>{label(event.event_type)}</strong><CopyControl value={event.event_id} label={`Copy event ${event.event_id}`} /></td><td><StatusBadge tone={stateTone(event.state)}>{label(event.state)}</StatusBadge></td><td><code>{event.aggregate_type}</code><br/><code>{event.aggregate_id}</code>{(event.transfer_id || event.account_id) && <nav className="event-related-links" aria-label={`Related evidence for ${event.event_id}`}>{event.transfer_id && <RecordLink href={`/transfers/${encodeURIComponent(event.transfer_id)}?return_to=${encodeURIComponent(returnTo)}`} label="Transfer" />}{event.account_id && <RecordLink href={`/accounts/${encodeURIComponent(event.account_id)}?return_to=${encodeURIComponent(returnTo)}`} label="Account" />}</nav>}</td><td>{event.attempt_count}</td><td><time dateTime={event.occurred_at}>{utc(event.occurred_at)}</time></td><td><RecordLink href={`/events/${encodeURIComponent(event.event_id)}?return_to=${encodeURIComponent(returnTo)}`} label="Open event" /></td></tr>)}</tbody></table></DataTableRegion><div className="pagination"><span>{nextHref ? "More event evidence is available" : "End of available event records"}</span>{nextHref ? <Link className="button secondary guarded-control" href={nextHref}>Next page</Link> : <button className="button secondary guarded-control" type="button" disabled>Next page</button>}</div></section>}
  </>;
}

export function EventDetailView({ event, verifiedAt, loading, error, online, canRead, returnTo, onRefresh }: Readonly<{ event: DeliveryEventDetail | null; verifiedAt?: string; loading: boolean; error: string | null; online: boolean; canRead: boolean; returnTo?: string; onRefresh: () => void }>) {
  const deliveryTone = event?.state === "published" ? "published" : event?.state === "dead" ? "dead" : "pending";
  return <>
    <Link className="record-link back-link" href={returnTo??"/events"}><ArrowLeft aria-hidden="true"/>Back to previous view</Link>
    <PageHeader eyebrow="Delivery evidence / Read-only detail" title="Event detail" description="Inspect a bounded delivery timeline and sanitized attempt evidence."><button className="button secondary guarded-control" type="button" disabled={!online || loading || !canRead} onClick={onRefresh}><ArrowClockwise aria-hidden="true"/>Refresh event</button></PageHeader>
    {!canRead && <StatePanel kind="denied" title="Event evidence not authorized" message="This session does not include events:read. No event detail has been requested." />}
    {!online && <StatePanel kind="offline" title="Offline — detail is not current" message="The last verified detail may remain visible. Reconnect before treating it as current." />}
    {error && <StatePanel kind="error" title="Event detail unavailable" message={error} />}
    {verifiedAt && event && <EvidenceFreshness state={error || !online ? "historical" : loading ? "refreshing" : "current"} verifiedAt={verifiedAt} label="Event detail" reason={error ?? (!online ? "Reconnect before relying on delivery state." : undefined)} />}
    {loading && !event && <StatePanel title="Loading event detail" message="Requesting authorized event evidence. No delivery or financial status is being inferred." />}
    {event && <article className="event-detail-document" aria-labelledby="event-detail-heading" aria-busy={loading}>
      <header><div><p className="eyebrow">Event evidence</p><h2 id="event-detail-heading">{label(event.event_type)}</h2><CopyControl value={event.event_id} label="Copy event ID" /></div><StatusBadge tone={stateTone(event.state)}>{label(event.state)}</StatusBadge></header>
      <div className={`delivery-impact-statement ${deliveryTone}`}>{deliveryTone === "published" ? <CheckCircle weight="fill" aria-hidden="true"/> : <WarningCircle weight="fill" aria-hidden="true"/>}<div><strong>{deliveryTone === "dead" ? "Delivery stopped; financial status is not inferred" : deliveryTone === "published" ? "Delivery published; financial status remains separate" : "Delivery is not confirmed; financial status is separate"}</strong><p>{event.transfer_id ? "A transfer may already be posted in PostgreSQL even when this event is not published. Open the transfer evidence to verify the financial result." : "This event state describes downstream delivery. It does not prove a balance or posting state."}</p></div></div>
      <dl className="evidence-list"><div><dt>Aggregate</dt><dd>{event.aggregate_type} · version {event.aggregate_version}</dd></div><div><dt>Aggregate ID</dt><dd><CopyControl value={event.aggregate_id} label="Copy aggregate ID" /></dd></div><div><dt>Attempt count</dt><dd>{event.attempt_count}</dd></div><div><dt>Occurred</dt><dd>{utc(event.occurred_at)}</dd></div><div><dt>Available</dt><dd>{utc(event.available_at)}</dd></div>{event.correlation_id && <div><dt>Correlation ID</dt><dd><CopyControl value={event.correlation_id} label="Copy correlation ID" /></dd></div>}{event.last_error_code && <div><dt>Last bounded error code</dt><dd><code>{event.last_error_code}</code></dd></div>}</dl>
      {(event.transfer_id || event.account_id) && <nav className="related-evidence" aria-label="Authorized related evidence">{event.transfer_id && <RecordLink href={`/transfers/${encodeURIComponent(event.transfer_id)}`} label="Open transfer evidence" />}{event.account_id && <RecordLink href={`/accounts/${encodeURIComponent(event.account_id)}`} label="Open account evidence" />}</nav>}
      <section className="event-timeline" aria-labelledby="event-timeline-heading"><div className="section-heading"><div><p className="eyebrow">Ordered evidence</p><h3 id="event-timeline-heading">Event timeline</h3></div></div>{event.timeline.length === 0 ? <StatePanel title="No timeline entries available" message="No additional timeline evidence was returned for this event." /> : <ol>{event.timeline.map((item, index) => <li key={`${item.kind}-${item.occurred_at}-${index}`}><span aria-hidden="true"><Clock weight="fill"/></span><div><strong>{label(item.kind)}</strong><time dateTime={item.occurred_at}>{utc(item.occurred_at)}</time></div></li>)}</ol>}</section>
      <section className="ledger-section" aria-labelledby="delivery-attempts-heading"><div className="section-heading"><div><p className="eyebrow">Sanitized downstream evidence</p><h3 id="delivery-attempts-heading">Delivery attempts</h3></div></div>{event.delivery_attempts_truncated && <StatePanel kind="unknown" title="Older attempts are not shown" message="This detail contains the newest bounded attempt evidence. The attempt count above may be larger than the rows shown."/>}{event.delivery_attempts.length === 0 ? <StatePanel title="No delivery attempts recorded" message="The event has no bounded attempt evidence yet." /> : <DataTableRegion label="Event delivery attempts"><table className="data-table"><thead><tr><th scope="col">Attempt</th><th scope="col">State</th><th scope="col">Destination</th><th scope="col">Due</th><th scope="col">Completed</th><th scope="col">Result class</th></tr></thead><tbody>{event.delivery_attempts.map((attempt) => <tr key={attempt.attempt_id}><td><strong>Attempt {attempt.attempt_number}</strong><CopyControl value={attempt.attempt_id} label={`Copy attempt ${attempt.attempt_number} ID`} /></td><td>{label(attempt.state)}</td><td>{attempt.endpoint_id ? <><strong>{attempt.endpoint_label}</strong><br/><code>{attempt.endpoint_origin}</code><br/><RecordLink href={`/webhooks/${encodeURIComponent(attempt.endpoint_id)}?return_to=${encodeURIComponent(`/events/${event.event_id}`)}`} label="Open endpoint" /></> : label(attempt.kind)}</td><td>{utc(attempt.due_at)}</td><td>{utc(attempt.completed_at)}</td><td><code>{attempt.response_class ?? attempt.error_code ?? "Not available"}</code></td></tr>)}</tbody></table></DataTableRegion>}</section>
    </article>}
    {event && <RelatedEvidenceRail sourceType="event" sourceId={event.event_id} />}
  </>;
}
