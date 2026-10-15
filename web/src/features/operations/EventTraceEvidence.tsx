import { CopyControl } from "@/ui/controls/CopyControl.client";
import { DisclosureSection } from "@/ui/disclosure/DisclosureSection";
import type { DeliveryEventDetail } from "@/lib/api/operations";

export function EventTraceEvidence({ event }: Readonly<{ event: DeliveryEventDetail }>) {
  return <DisclosureSection id="event-diagnostic-trace" title="Diagnostic trace" summary="Private diagnostic context; separate from financial proof." lazy>
    {event.diagnostic_trace_id ? <>
      <p>This is the saved origin trace. Sampling or retention can make a trace unavailable. A trace does not prove a financial result.</p>
      <CopyControl value={event.diagnostic_trace_id} label="Copy diagnostic trace ID" />
      {event.trace_viewer_available ? <a className="text-link" href={`/api/events/${encodeURIComponent(event.event_id)}/trace`} target="_blank" rel="noopener noreferrer">Open private trace viewer</a>
        : <p>The private trace viewer is not configured. Keep this ID for an authorized diagnostic review.</p>}
    </> : <p>No diagnostic origin was recorded for this event. Historical events remain valid financial evidence.</p>}
  </DisclosureSection>;
}
