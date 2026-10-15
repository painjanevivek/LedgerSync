import type { PresentationTone } from "@/features/console/presentation";
import type { FundingEvent } from "@/lib/api/funding";
import type { TransferCorrection } from "@/lib/api/corrections";
import type { DeliveryEvent, WebhookEndpoint } from "@/lib/api/operations";
import type { RecoveryEvidenceIndex } from "@/lib/api/recovery";

export type TaskCoverage = Readonly<{ id: string; label: string; state: "loading" | "verified" | "partial" | "unavailable" | "not-authorized"; href: string }>;
export type WorkspaceTask = Readonly<{ id: string; title: string; explanation: string; tone: PresentationTone; priority: number; actionable: boolean; occurredAt?: string; amountMinor?: string; currency?: string; reference?: string; group: "attention" | "history" | "setup"; action: { label: string; href: string } }>;

export function orderTasks(tasks: readonly WorkspaceTask[]): WorkspaceTask[] {
  const sorted = [...tasks].sort((left, right) => left.priority - right.priority || Number(right.actionable) - Number(left.actionable) || (Date.parse(left.occurredAt ?? "") || Infinity) - (Date.parse(right.occurredAt ?? "") || Infinity) || left.id.localeCompare(right.id));
  const unique = new Map<string, WorkspaceTask>();
  for (const task of sorted) if (!unique.has(task.id)) unique.set(task.id, task);
  return [...unique.values()];
}

export function fundingTasks(records: readonly FundingEvent[]): WorkspaceTask[] {
  return records.filter(record => record.status === "requested" || record.status === "approved").map(record => ({ id: `funding:${record.funding_event_id}`, title: record.status === "requested" ? "Review money being added" : "Confirm the funding result", explanation: record.status === "requested" ? "This money is awaiting review. It is not yet available to use." : "Approval alone is not proof of credit. Open the record to check its final result.", tone: "warning", priority: 2, actionable: false, occurredAt: record.requested_at, amountMinor: record.amount_minor, currency: record.currency, reference: record.funding_event_id, group: "attention", action: { label: "Open funding record", href: `/funding/${record.funding_event_id}?return_to=%2Ftasks` } }));
}

export function correctionTasks(records: readonly TransferCorrection[]): WorkspaceTask[] {
  return records.filter(record => record.status === "requested" || record.status === "approved").map(record => ({ id: `correction:${record.correction_id}`, title: record.status === "approved" ? "An approved correction needs execution" : "Review a record correction", explanation: record.status === "approved" ? "Approval has not moved money. Check the original and offsetting movements before execution." : "An independent operator must review the proposed correction before it can proceed.", tone: "warning", priority: 2, actionable: false, occurredAt: record.requested_at, amountMinor: record.amount_minor, currency: record.currency, reference: record.correction_id, group: "attention", action: { label: "Review correction", href: `/corrections/${record.correction_id}?return_to=%2Ftasks` } }));
}

export function deliveryTasks(records: readonly DeliveryEvent[]): WorkspaceTask[] {
  return records.filter(record => record.state === "dead" || record.state === "unknown").map(record => ({ id: `delivery:${record.event_id}`, title: "A delivery needs attention", explanation: "A message could not be confirmed as delivered. This does not mean the money movement failed.", tone: "warning", priority: 3, actionable: false, occurredAt: record.dead_at ?? record.occurred_at, reference: record.event_id, group: "attention", action: { label: "Review delivery", href: `/events/${record.event_id}` } }));
}

export function webhookTasks(records: readonly WebhookEndpoint[]): WorkspaceTask[] {
  return records.filter(record => record.recent_delivery_state === "dead" || record.recent_delivery_state === "unknown").map(record => ({ id: `webhook:${record.endpoint_id}`, title: "Check a delivery destination", explanation: "Recent delivery to this destination needs review. Review attempts before requesting a replay; replaying delivery does not move money.", tone: "warning", priority: 3, actionable: false, occurredAt: record.latest_delivery_at ?? record.updated_at, reference: record.endpoint_id, group: "attention", action: { label: "Review destination", href: `/webhooks/${record.endpoint_id}` } }));
}

export function recoveryTasks(index: RecoveryEvidenceIndex): WorkspaceTask[] {
  if (index.latest_backup && index.latest_restore) return [];
  return [{ id: "setup:recovery", title: "Recovery evidence is not complete", explanation: "A verified backup and restore check are not both recorded. This is a setup gap, not evidence of a failed transfer.", tone: "neutral", priority: 5, actionable: false, occurredAt: index.generated_at_utc, group: "setup", action: { label: "View recovery guidance", href: "/recovery" } }];
}
