"use client";
import type { DeliveryEventDetail } from "@/lib/api/operations";
import { CopyControl } from "@/ui/controls/CopyControl.client";
import { DataTableRegion } from "@/ui/display/DataTableRegion";
import { Timestamp } from "@/ui/display/Timestamp";
export function EventOrderEvidence({ event }: Readonly<{ event: DeliveryEventDetail }>) {
  return <section aria-label="Logical event order">
    {event.event_order ? <dl className="evidence-list"><div><dt>Origin node</dt><dd><CopyControl value={event.event_order.node_id} /></dd></div><div><dt>Logical timestamp</dt><dd><code>{event.event_order.logical_timestamp}</code></dd></div>{event.event_order.parent_reference && <div><dt>Parent reference</dt><dd><CopyControl value={event.event_order.parent_reference} /></dd></div>}</dl> : <p>Causal metadata is unavailable for this older event.</p>}
    <p>Receive clocks exceed send clocks. A larger clock alone does not prove causation. Aggregate versions still control cache order.</p>
    <h3>Publication attempts</h3>
    {event.publication_attempts_truncated && <p>Only the latest 25 attempts are shown.</p>}
    {!event.publication_attempts ? <p>Publication evidence unavailable.</p> : event.publication_attempts.length === 0 ? <p>No publication attempts recorded.</p> : <DataTableRegion label="Publication clock evidence"><table className="data-table"><thead><tr><th scope="col">Generation</th><th scope="col">Worker</th><th scope="col">State</th><th scope="col">Receive / send clock</th><th scope="col">Started</th></tr></thead><tbody>{event.publication_attempts.map((attempt) => <tr key={attempt.generation}><td>{attempt.generation}</td><td><CopyControl value={attempt.worker_id} /></td><td>{attempt.state}</td><td>{attempt.receive_counter ?? "Unavailable"} / {attempt.send_counter ?? "Unavailable"}</td><td><Timestamp value={attempt.started_at} /></td></tr>)}</tbody></table></DataTableRegion>}
    <h3>Receive clocks</h3>
    {event.receipts_truncated && <p>Only the latest 25 receipts are shown.</p>}
    {!event.receipts ? <p>Receive evidence unavailable.</p> : event.receipts.length === 0 ? <p>No receipts recorded.</p> : <DataTableRegion label="Received message clocks"><table className="data-table"><thead><tr><th scope="col">Node</th><th scope="col">Sender</th><th scope="col">Send / receive clock</th><th scope="col">Received</th></tr></thead><tbody>{event.receipts.map((receipt) => <tr key={`${receipt.node_id}/${receipt.receive_counter}`}><td><CopyControl value={receipt.node_id} /></td><td>{receipt.sender_node_id ?? "Older envelope"}</td><td>{receipt.send_counter ?? "Unavailable"} / {receipt.receive_counter}</td><td><Timestamp value={receipt.received_at} /></td></tr>)}</tbody></table></DataTableRegion>}
  </section>;
}
