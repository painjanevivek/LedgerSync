import { isRFC3339Timestamp, parseStrictListQuery, parseStrictListSearchParams, type StrictListQueryInput } from "@/lib/strict-list-query";

export type EventFilters = Readonly<{ eventType?: string; state?: "pending" | "retrying" | "published" | "dead"; endpointId?: string; relatedId?: string; correlationId?: string; from?: string; to?: string; cursor?: string }>;
export type WebhookFilters = Readonly<{ status?: "pending_verification" | "active" | "disabled"; eventType?: string; cursor?: string }>;

export const emptyEventFilters: EventFilters = {};
export const emptyWebhookFilters: WebhookFilters = {};

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const eventType = /^[A-Za-z0-9][A-Za-z0-9._:-]*$/;

export const eventPageQueryRules = {
  eventType: { maximumLength: 256, pattern: eventType },
  state: { maximumLength: 16, values: ["pending", "retrying", "published", "dead"] },
  endpointId: { maximumLength: 36, pattern: uuid },
  relatedId: { maximumLength: 36, pattern: uuid },
  correlationId: { maximumLength: 36, pattern: uuid },
  from: { maximumLength: 64, validate: isRFC3339Timestamp },
  to: { maximumLength: 64, validate: isRFC3339Timestamp },
  cursor: { maximumLength: 2_048 },
} as const;

export const eventBFFQueryRules = {
  ...eventPageQueryRules,
  limit: { maximumLength: 3, pattern: /^(?:[1-9]|[1-9][0-9]|100)$/ },
} as const;

export const webhookPageQueryRules = {
  status: { maximumLength: 32, values: ["pending_verification", "active", "disabled"] },
  eventType: { maximumLength: 128, pattern: eventType },
  cursor: { maximumLength: 2_048 },
} as const;

export const webhookBFFQueryRules = {
  ...webhookPageQueryRules,
  limit: { maximumLength: 3, pattern: /^(?:[1-9]|[1-9][0-9]|100)$/ },
} as const;

function validRange(values: Readonly<Partial<Record<string, string>>>) {
  return !(values.from && values.to && Date.parse(values.from) > Date.parse(values.to));
}

function canonicalRFC3339(value: string) {
  return new Date(value).toISOString().replace(".000Z", "Z");
}

export function parseEventPageQuery(input: StrictListQueryInput): Readonly<{ ok: true; filters: EventFilters }> | Readonly<{ ok: false }> {
  const result = parseStrictListQuery(input, eventPageQueryRules);
  if (!result.ok || !validRange(result.values)) return { ok: false };
  const values = result.values;
  return { ok: true, filters: {
    eventType: values.eventType,
    state: values.state as EventFilters["state"],
    endpointId: values.endpointId?.toLowerCase(),
    relatedId: values.relatedId?.toLowerCase(),
    correlationId: values.correlationId?.toLowerCase(),
    from: values.from ? canonicalRFC3339(values.from) : undefined,
    to: values.to ? canonicalRFC3339(values.to) : undefined,
    cursor: values.cursor,
  } };
}

export function parseWebhookPageQuery(input: StrictListQueryInput): Readonly<{ ok: true; filters: WebhookFilters }> | Readonly<{ ok: false }> {
  const result = parseStrictListQuery(input, webhookPageQueryRules);
  if (!result.ok) return { ok: false };
  return { ok: true, filters: { status: result.values.status as WebhookFilters["status"], eventType: result.values.eventType, cursor: result.values.cursor } };
}

export function parseEventBFFSearchParams(searchParams: URLSearchParams) {
  const result = parseStrictListSearchParams(searchParams, eventBFFQueryRules);
  if (!result.ok || !validRange(result.values)) return { ok: false } as const;
  return result;
}

export function parseWebhookBFFSearchParams(searchParams: URLSearchParams) {
  return parseStrictListSearchParams(searchParams, webhookBFFQueryRules);
}

function listURL<T extends object>(path: string, filters: T) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) if (value) query.set(key, value);
  return query.size ? `${path}?${query}` : path;
}

export function eventsURL(filters: EventFilters) { return listURL("/events", filters); }
export function webhooksURL(filters: WebhookFilters) { return listURL("/webhooks", filters); }
