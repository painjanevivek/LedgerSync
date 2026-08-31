"use client";

import { useFinancialPostCommand } from "@/features/console/useFinancialPostCommand";
import type { FundingEvent } from "@/lib/api/funding";

function parseFundingPost(value: unknown): FundingEvent | null {
  if (!value || Array.isArray(value) || typeof value !== "object") return null;
  const record = value as { event?: unknown };
  const candidate = record.event;
  if (!candidate || Array.isArray(candidate) || typeof candidate !== "object") return null;
  const event = candidate as Partial<FundingEvent>;
  return typeof event.funding_event_id === "string"
    && (event.status === "posted" || event.status === "compensated")
    && typeof event.amount_minor === "string"
    && typeof event.currency === "string"
    ? event as FundingEvent
    : null;
}

export function useFundingCommand(tenantId: string, eventId: string, csrfToken: string) {
  return useFinancialPostCommand<FundingEvent>({
    domain: "funding",
    tenantId,
    recordId: eventId,
    csrfToken,
    endpoint: `/api/funding-events/${encodeURIComponent(eventId)}/post`,
    parseSuccess: parseFundingPost,
  });
}
