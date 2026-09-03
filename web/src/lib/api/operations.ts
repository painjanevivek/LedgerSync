export type OperationalState = "ready" | "degraded" | "unavailable";
export type DependencyState = "reachable" | "unavailable";

export type LocalDiagnostics = Readonly<{
  overall_state: OperationalState;
  generated_at: string;
  application: Readonly<{ version: string; commit: string; environment: string; public_origin?: string }>;
  financial_authority: Readonly<{
    postgres: Readonly<{ state: DependencyState; schema_version?: string }>;
    latest_reconciliation: Readonly<{ state: "available" | "none" | "unavailable"; status?: string; run_id?: string; completed_at?: string }>;
  }>;
  delivery_cache: Readonly<{
    outbox: Readonly<{
      state: DependencyState;
      pending_count?: string;
      dead_count?: string;
      worker_progress: "recent" | "idle" | "stalled" | "unknown";
      latest_published_at?: string;
      oldest_pending_at?: string;
    }>;
    redis: Readonly<{ state: DependencyState; label: "disposable_cache" }>;
  }>;
}>;

export type DeliveryAttempt = Readonly<{
  attempt_id: string;
  kind: string;
  state: string;
  attempt_number: string;
  due_at: string;
  started_at?: string;
  completed_at?: string;
  response_class?: string;
  error_code?: string;
  endpoint_id?: string;
  endpoint_label?: string;
  endpoint_origin?: string;
}>;

export type WebhookEndpoint = Readonly<{
  endpoint_id: string;
  label: string;
  origin: string;
  status: "pending_verification" | "active" | "disabled" | "unknown";
  subscribed_events: string[];
  recent_delivery_state: "none" | "pending" | "retrying" | "delivered" | "dead" | "unknown";
  recent_attempt_count: string;
  recent_dead_count: string;
  verified_at?: string;
  disabled_at?: string;
  latest_delivery_at?: string;
  updated_at: string;
}>;

export type WebhookDeliveryAttempt = Readonly<{
  attempt_id: string;
  event_id?: string;
  transfer_id: string;
  state: "pending" | "retrying" | "delivered" | "dead" | "unknown";
  attempt_number: string;
  response_class?: string;
  error_code?: string;
  due_at: string;
  started_at?: string;
  completed_at?: string;
}>;

export type WebhookEndpointPage = Readonly<{ items: WebhookEndpoint[]; next_cursor: string }>;
export type WebhookEndpointDetail = WebhookEndpoint & Readonly<{ delivery_attempts: WebhookDeliveryAttempt[]; delivery_attempts_truncated: boolean }>;

export type EventTimelineItem = Readonly<{ kind: string; occurred_at: string }>;

export type DeliveryEvent = Readonly<{
  event_id: string;
  event_type: string;
  state: "pending" | "retrying" | "published" | "dead" | "unknown";
  aggregate_type: string;
  aggregate_id: string;
  aggregate_version: string;
  attempt_count: string;
  occurred_at: string;
  available_at: string;
  transfer_id?: string;
  account_id?: string;
  correlation_id?: string;
  claimed_until?: string;
  published_at?: string;
  dead_at?: string;
  last_error_code?: string;
}>;

export type DeliveryEventDetail = DeliveryEvent & Readonly<{
  delivery_attempts: DeliveryAttempt[];
  delivery_attempts_truncated: boolean;
  timeline: EventTimelineItem[];
}>;

export type EventPage = Readonly<{ events: DeliveryEvent[]; next_cursor: string }>;

export type SanitizedOperationsResponse = Readonly<{ status: number; body: Readonly<Record<string, unknown>> }>;

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const identifier = /^[A-Za-z0-9][A-Za-z0-9._:-]*$/;
const canonicalCount = /^(?:0|[1-9][0-9]*)$/;
const operationalStates = new Set(["ready", "degraded", "unavailable"]);
const dependencyStates = new Set(["reachable", "unavailable"]);
const eventStates = new Set(["pending", "retrying", "published", "dead", "unknown"]);
const attemptKinds = new Set(["webhook", "notification", "unknown"]);
const attemptStates = new Set(["pending", "retrying", "delivered", "dead", "unknown"]);
const webhookEndpointStatuses = new Set(["pending_verification", "active", "disabled", "unknown"]);
const webhookDeliveryStates = new Set(["none", "pending", "retrying", "delivered", "dead", "unknown"]);
const responseClasses = new Set(["2xx", "3xx", "4xx", "5xx", "network_error", "timeout", "redacted"]);
const eventErrorCodes = new Set(["publish_failed", "invalid_event", "redis_unavailable", "approved_replay", "redacted"]);
const attemptErrorCodes = new Set(["timeout", "publish_failed", "invalid_event", "redis_unavailable", "recipient_unavailable", "connection_failed", "redacted"]);
const timelineKinds = new Set(["committed", "available", "claim_lease_expires", "published", "dead", "delivery_started", "delivery_pending", "delivery_retrying", "delivery_delivered", "delivery_dead", "delivery_unknown"]);
const reconciliationStates = new Set(["available", "none", "unavailable"]);
const reconciliationStatuses = new Set(["matched", "mismatch", "failed"]);
const workerStates = new Set(["recent", "idle", "stalled", "unknown"]);
const maximumUpstreamBytes = 262_144;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function keysAllowed(value: Record<string, unknown>, allowed: readonly string[]): boolean {
  const names = new Set(allowed);
  return Object.keys(value).every((key) => names.has(key));
}

function bounded(value: unknown, maximum = 256, allowEmpty = false): value is string {
  return typeof value === "string" && value.length <= maximum && (allowEmpty || value.length > 0);
}

function safeIdentifier(value: unknown, maximum = 128): value is string {
  return bounded(value, maximum) && identifier.test(value);
}

function safeUUID(value: unknown): value is string {
  return typeof value === "string" && uuid.test(value);
}

function timestamp(value: unknown): value is string {
  return bounded(value, 64) && !Number.isNaN(Date.parse(value));
}

function optionalTimestamp(value: unknown): value is string | undefined {
  return value === undefined || timestamp(value);
}

function endpointOrigin(value: unknown): value is string {
  if (!bounded(value, 255)) return false;
  if (value === "unavailable") return true;
  try {
    const parsed = new URL(value);
    return (parsed.protocol === "https:" || parsed.protocol === "http:") && parsed.username === "" && parsed.password === "" && parsed.origin === value && parsed.pathname === "/" && parsed.search === "" && parsed.hash === "";
  } catch { return false; }
}

function count(value: unknown): string | null {
  if (typeof value === "string" && canonicalCount.test(value) && value.length <= 20) return value;
  if (typeof value === "number" && Number.isSafeInteger(value) && value >= 0) return String(value);
  return null;
}

function sanitizeError(status: number, value: unknown): SanitizedOperationsResponse {
  const error = isRecord(value) && isRecord(value.error) ? value.error : undefined;
  const code = typeof error?.code === "string" ? error.code : "";
  if (status === 400 && (code === "invalid_request" || code === "validation_failed")) return { status, body: { error: { code } } };
  if (status === 401 && code === "unauthorized") return { status, body: { error: { code } } };
  if (status === 403 && code === "forbidden") return { status, body: { error: { code } } };
  if (status === 404 && code === "not_found") return { status, body: { error: { code } } };
  if (status === 429 && code === "rate_limited") return { status, body: { error: { code } } };
  if (status === 503 && (code === "evidence_unavailable" || code === "temporary_unavailable")) return { status, body: { error: { code } } };
  if (status === 504 && code === "upstream_timeout") return { status, body: { error: { code } } };
  return { status: 503, body: { error: { code: "evidence_unavailable" } } };
}

function diagnostics(value: unknown, publicOrigin?: string): LocalDiagnostics | null {
  if (!isRecord(value) || !keysAllowed(value, ["overall_state", "generated_at", "application", "financial_authority", "delivery_cache"])
    || typeof value.overall_state !== "string" || !operationalStates.has(value.overall_state)
    || !timestamp(value.generated_at) || !isRecord(value.application) || !isRecord(value.financial_authority) || !isRecord(value.delivery_cache)) return null;
  const application = value.application;
  const financial = value.financial_authority;
  const delivery = value.delivery_cache;
  if (!keysAllowed(application, ["version", "commit", "environment"])
    || !bounded(application.version, 128) || !bounded(application.commit, 128) || !bounded(application.environment, 64)
    || !keysAllowed(financial, ["postgres", "latest_reconciliation"])
    || !isRecord(financial.postgres) || !isRecord(financial.latest_reconciliation)
    || !keysAllowed(delivery, ["outbox", "redis"]) || !isRecord(delivery.outbox) || !isRecord(delivery.redis)) return null;
  const postgres = financial.postgres;
  const reconciliation = financial.latest_reconciliation;
  const outbox = delivery.outbox;
  const redis = delivery.redis;
  const pending = outbox.pending_count === undefined ? null : count(outbox.pending_count);
  const dead = outbox.dead_count === undefined ? null : count(outbox.dead_count);
  if (!keysAllowed(postgres, ["state", "schema_version"])
    || typeof postgres.state !== "string" || !dependencyStates.has(postgres.state)
    || postgres.schema_version !== undefined && !bounded(postgres.schema_version, 128)
    || postgres.state === "reachable" && postgres.schema_version === undefined
    || !keysAllowed(reconciliation, ["state", "status", "run_id", "completed_at"])
    || typeof reconciliation.state !== "string" || !reconciliationStates.has(reconciliation.state)
    || reconciliation.status !== undefined && (typeof reconciliation.status !== "string" || !reconciliationStatuses.has(reconciliation.status))
    || reconciliation.run_id !== undefined && !safeUUID(reconciliation.run_id)
    || !optionalTimestamp(reconciliation.completed_at)
    || !keysAllowed(outbox, ["state", "pending_count", "dead_count", "worker_progress", "latest_published_at", "oldest_pending_at"])
    || typeof outbox.state !== "string" || !dependencyStates.has(outbox.state)
    || outbox.state === "reachable" && (pending === null || dead === null)
    || outbox.state === "unavailable" && (outbox.pending_count !== undefined && pending === null || outbox.dead_count !== undefined && dead === null)
    || typeof outbox.worker_progress !== "string" || !workerStates.has(outbox.worker_progress)
    || !optionalTimestamp(outbox.latest_published_at) || !optionalTimestamp(outbox.oldest_pending_at)
    || !keysAllowed(redis, ["state", "label"])
    || typeof redis.state !== "string" || !dependencyStates.has(redis.state)
    || redis.label !== "disposable_cache") return null;
  if (reconciliation.state === "available" && (reconciliation.status === undefined || reconciliation.run_id === undefined || reconciliation.completed_at === undefined)) return null;
  if (reconciliation.state !== "available" && (reconciliation.status !== undefined || reconciliation.run_id !== undefined || reconciliation.completed_at !== undefined)) return null;
  const readyContradiction = value.overall_state === "ready" && (
    postgres.state !== "reachable" || reconciliation.state !== "available" || outbox.state !== "reachable" || redis.state !== "reachable"
    || dead !== "0" || outbox.worker_progress === "stalled" || reconciliation.status === "mismatch" || reconciliation.status === "failed"
  );
  const databaseAvailabilityContradiction = postgres.state === "unavailable" && value.overall_state !== "unavailable"
    || postgres.state === "reachable" && value.overall_state === "unavailable";
  if (readyContradiction || databaseAvailabilityContradiction) return null;
  return {
    overall_state: value.overall_state as LocalDiagnostics["overall_state"],
    generated_at: value.generated_at,
    application: { version: application.version, commit: application.commit, environment: application.environment, ...(publicOrigin ? { public_origin: publicOrigin } : {}) },
    financial_authority: {
      postgres: { state: postgres.state as DependencyState, ...(postgres.schema_version === undefined ? {} : { schema_version: postgres.schema_version }) },
      latest_reconciliation: {
        state: reconciliation.state as LocalDiagnostics["financial_authority"]["latest_reconciliation"]["state"],
        ...(reconciliation.status === undefined ? {} : { status: reconciliation.status }),
        ...(reconciliation.run_id === undefined ? {} : { run_id: reconciliation.run_id }),
        ...(reconciliation.completed_at === undefined ? {} : { completed_at: reconciliation.completed_at }),
      },
    },
    delivery_cache: {
      outbox: {
        state: outbox.state as DependencyState,
        ...(pending === null ? {} : { pending_count: pending }),
        ...(dead === null ? {} : { dead_count: dead }),
        worker_progress: outbox.worker_progress as LocalDiagnostics["delivery_cache"]["outbox"]["worker_progress"],
        ...(outbox.latest_published_at === undefined ? {} : { latest_published_at: outbox.latest_published_at }),
        ...(outbox.oldest_pending_at === undefined ? {} : { oldest_pending_at: outbox.oldest_pending_at }),
      },
      redis: { state: redis.state as DependencyState, label: "disposable_cache" },
    },
  };
}

function event(value: unknown): DeliveryEvent | null {
  if (!isRecord(value) || !keysAllowed(value, ["event_id", "event_type", "state", "aggregate_type", "aggregate_id", "aggregate_version", "attempt_count", "occurred_at", "available_at", "transfer_id", "account_id", "correlation_id", "claimed_until", "published_at", "dead_at", "last_error_code"])) return null;
  const version = count(value.aggregate_version);
  const attempts = count(value.attempt_count);
  if (!safeUUID(value.event_id) || !safeIdentifier(value.event_type) || typeof value.state !== "string" || !eventStates.has(value.state)
    || !safeIdentifier(value.aggregate_type, 64) || !safeUUID(value.aggregate_id) || version === null || attempts === null
    || !timestamp(value.occurred_at) || !timestamp(value.available_at)
    || value.transfer_id !== undefined && !safeUUID(value.transfer_id)
    || value.account_id !== undefined && !safeUUID(value.account_id)
    || value.correlation_id !== undefined && !safeIdentifier(value.correlation_id, 128)
    || !optionalTimestamp(value.claimed_until) || !optionalTimestamp(value.published_at) || !optionalTimestamp(value.dead_at)
    || value.last_error_code !== undefined && (typeof value.last_error_code !== "string" || !eventErrorCodes.has(value.last_error_code))) return null;
  return {
    event_id: value.event_id,
    event_type: value.event_type,
    state: value.state as DeliveryEvent["state"],
    aggregate_type: value.aggregate_type,
    aggregate_id: value.aggregate_id,
    aggregate_version: version,
    attempt_count: attempts,
    occurred_at: value.occurred_at,
    available_at: value.available_at,
    ...(value.transfer_id === undefined ? {} : { transfer_id: value.transfer_id }),
    ...(value.account_id === undefined ? {} : { account_id: value.account_id }),
    ...(value.correlation_id === undefined ? {} : { correlation_id: value.correlation_id }),
    ...(value.claimed_until === undefined ? {} : { claimed_until: value.claimed_until }),
    ...(value.published_at === undefined ? {} : { published_at: value.published_at }),
    ...(value.dead_at === undefined ? {} : { dead_at: value.dead_at }),
    ...(value.last_error_code === undefined ? {} : { last_error_code: value.last_error_code }),
  };
}

function attempt(value: unknown): DeliveryAttempt | null {
  if (!isRecord(value) || !keysAllowed(value, ["attempt_id", "kind", "state", "attempt_number", "due_at", "started_at", "completed_at", "response_class", "error_code", "endpoint_id", "endpoint_label", "endpoint_origin"])) return null;
  const number = count(value.attempt_number);
  if (!safeUUID(value.attempt_id) || typeof value.kind !== "string" || !attemptKinds.has(value.kind) || typeof value.state !== "string" || !attemptStates.has(value.state) || number === null || !timestamp(value.due_at)
    || !optionalTimestamp(value.started_at) || !optionalTimestamp(value.completed_at)
    || value.response_class !== undefined && (typeof value.response_class !== "string" || !responseClasses.has(value.response_class))
    || value.error_code !== undefined && (typeof value.error_code !== "string" || !attemptErrorCodes.has(value.error_code))
    || value.endpoint_id !== undefined && !safeUUID(value.endpoint_id)
    || value.endpoint_label !== undefined && !bounded(value.endpoint_label, 100)
    || value.endpoint_origin !== undefined && !endpointOrigin(value.endpoint_origin)
    || value.endpoint_id === undefined && (value.endpoint_label !== undefined || value.endpoint_origin !== undefined)
    || value.endpoint_id !== undefined && (value.endpoint_label === undefined || value.endpoint_origin === undefined)) return null;
  return {
    attempt_id: value.attempt_id, kind: value.kind, state: value.state, attempt_number: number, due_at: value.due_at,
    ...(value.started_at === undefined ? {} : { started_at: value.started_at }),
    ...(value.completed_at === undefined ? {} : { completed_at: value.completed_at }),
    ...(value.response_class === undefined ? {} : { response_class: value.response_class }),
    ...(value.error_code === undefined ? {} : { error_code: value.error_code }),
    ...(value.endpoint_id === undefined ? {} : { endpoint_id: value.endpoint_id, endpoint_label: value.endpoint_label as string, endpoint_origin: value.endpoint_origin as string }),
  };
}

function webhookEndpoint(value: unknown): WebhookEndpoint | null {
  if (!isRecord(value) || !keysAllowed(value, ["endpoint_id", "label", "origin", "status", "subscribed_events", "recent_delivery_state", "recent_attempt_count", "recent_dead_count", "verified_at", "disabled_at", "latest_delivery_at", "updated_at"])) return null;
  const recentAttempts = count(value.recent_attempt_count);
  const recentDead = count(value.recent_dead_count);
  if (!safeUUID(value.endpoint_id) || !bounded(value.label, 100) || !endpointOrigin(value.origin)
    || typeof value.status !== "string" || !webhookEndpointStatuses.has(value.status)
    || !Array.isArray(value.subscribed_events) || value.subscribed_events.length < 1 || value.subscribed_events.length > 32 || !value.subscribed_events.every((item) => safeIdentifier(item, 128))
    || typeof value.recent_delivery_state !== "string" || !webhookDeliveryStates.has(value.recent_delivery_state)
    || recentAttempts === null || recentDead === null || BigInt(recentDead) > BigInt(recentAttempts)
    || !optionalTimestamp(value.verified_at) || !optionalTimestamp(value.disabled_at) || !optionalTimestamp(value.latest_delivery_at) || !timestamp(value.updated_at)
    || value.status === "active" && value.verified_at === undefined
    || value.status === "disabled" && value.disabled_at === undefined) return null;
  return {
    endpoint_id: value.endpoint_id, label: value.label, origin: value.origin,
    status: value.status as WebhookEndpoint["status"], subscribed_events: [...value.subscribed_events] as string[],
    recent_delivery_state: value.recent_delivery_state as WebhookEndpoint["recent_delivery_state"], recent_attempt_count: recentAttempts, recent_dead_count: recentDead,
    ...(value.verified_at === undefined ? {} : { verified_at: value.verified_at }),
    ...(value.disabled_at === undefined ? {} : { disabled_at: value.disabled_at }),
    ...(value.latest_delivery_at === undefined ? {} : { latest_delivery_at: value.latest_delivery_at }),
    updated_at: value.updated_at,
  };
}

function webhookDeliveryAttempt(value: unknown): WebhookDeliveryAttempt | null {
  if (!isRecord(value) || !keysAllowed(value, ["attempt_id", "event_id", "transfer_id", "state", "attempt_number", "response_class", "error_code", "due_at", "started_at", "completed_at"])) return null;
  const number = count(value.attempt_number);
  if (!safeUUID(value.attempt_id) || value.event_id !== undefined && !safeUUID(value.event_id) || !safeUUID(value.transfer_id)
    || typeof value.state !== "string" || !attemptStates.has(value.state) || number === null || !timestamp(value.due_at)
    || !optionalTimestamp(value.started_at) || !optionalTimestamp(value.completed_at)
    || value.response_class !== undefined && (typeof value.response_class !== "string" || !responseClasses.has(value.response_class))
    || value.error_code !== undefined && (typeof value.error_code !== "string" || !attemptErrorCodes.has(value.error_code))) return null;
  return {
    attempt_id: value.attempt_id, ...(value.event_id === undefined ? {} : { event_id: value.event_id }), transfer_id: value.transfer_id,
    state: value.state as WebhookDeliveryAttempt["state"], attempt_number: number,
    ...(value.response_class === undefined ? {} : { response_class: value.response_class }),
    ...(value.error_code === undefined ? {} : { error_code: value.error_code }),
    due_at: value.due_at, ...(value.started_at === undefined ? {} : { started_at: value.started_at }), ...(value.completed_at === undefined ? {} : { completed_at: value.completed_at }),
  };
}

function timelineItem(value: unknown): EventTimelineItem | null {
  if (!isRecord(value) || !keysAllowed(value, ["kind", "occurred_at"]) || typeof value.kind !== "string" || !timelineKinds.has(value.kind) || !timestamp(value.occurred_at)) return null;
  return { kind: value.kind, occurred_at: value.occurred_at };
}

export function sanitizeLocalDiagnostics(status: number, value: unknown, publicOrigin?: string): SanitizedOperationsResponse {
  if (status < 200 || status >= 300) return sanitizeError(status, value);
  const result = diagnostics(value, publicOrigin);
  return result ? { status: 200, body: result } : { status: 503, body: { error: { code: "evidence_unavailable" } } };
}

export function sanitizeEventPage(status: number, value: unknown): SanitizedOperationsResponse {
  if (status < 200 || status >= 300) return sanitizeError(status, value);
  if (!isRecord(value) || !keysAllowed(value, ["events", "next_cursor"]) || !Array.isArray(value.events) || value.events.length > 100 || !bounded(value.next_cursor, 2_048, true)) return { status: 503, body: { error: { code: "evidence_unavailable" } } };
  const events = value.events.map(event);
  if (events.some((item) => item === null)) return { status: 503, body: { error: { code: "evidence_unavailable" } } };
  return { status: 200, body: { events, next_cursor: value.next_cursor } };
}

export function sanitizeEventDetail(status: number, value: unknown): SanitizedOperationsResponse {
  if (status < 200 || status >= 300) return sanitizeError(status, value);
  if (!isRecord(value) || !keysAllowed(value, ["event_id", "event_type", "state", "aggregate_type", "aggregate_id", "aggregate_version", "attempt_count", "occurred_at", "available_at", "transfer_id", "account_id", "correlation_id", "claimed_until", "published_at", "dead_at", "last_error_code", "delivery_attempts", "delivery_attempts_truncated", "timeline"])) return { status: 503, body: { error: { code: "evidence_unavailable" } } };
  const { delivery_attempts, delivery_attempts_truncated, timeline, ...baseValue } = value;
  const base = event(baseValue);
  if (!base || !Array.isArray(delivery_attempts) || delivery_attempts.length > 25 || typeof delivery_attempts_truncated !== "boolean" || !Array.isArray(timeline) || timeline.length > 32) return { status: 503, body: { error: { code: "evidence_unavailable" } } };
  const attempts = delivery_attempts.map(attempt);
  const items = timeline.map(timelineItem);
  if (attempts.some((item) => item === null) || items.some((item) => item === null)) return { status: 503, body: { error: { code: "evidence_unavailable" } } };
  return { status: 200, body: { ...base, delivery_attempts: attempts, delivery_attempts_truncated, timeline: items } };
}

export function sanitizeWebhookEndpointPage(status: number, value: unknown): SanitizedOperationsResponse {
  if (status < 200 || status >= 300) return sanitizeError(status, value);
  if (!isRecord(value) || !keysAllowed(value, ["items", "next_cursor"]) || !Array.isArray(value.items) || value.items.length > 100 || !bounded(value.next_cursor, 2_048, true)) return { status: 503, body: { error: { code: "evidence_unavailable" } } };
  const items = value.items.map(webhookEndpoint);
  if (items.some((item) => item === null)) return { status: 503, body: { error: { code: "evidence_unavailable" } } };
  return { status: 200, body: { items, next_cursor: value.next_cursor } };
}

export function sanitizeWebhookEndpointDetail(status: number, value: unknown): SanitizedOperationsResponse {
  if (status < 200 || status >= 300) return sanitizeError(status, value);
  if (!isRecord(value) || !keysAllowed(value, ["endpoint_id", "label", "origin", "status", "subscribed_events", "recent_delivery_state", "recent_attempt_count", "recent_dead_count", "verified_at", "disabled_at", "latest_delivery_at", "updated_at", "delivery_attempts", "delivery_attempts_truncated"])) return { status: 503, body: { error: { code: "evidence_unavailable" } } };
  const { delivery_attempts, delivery_attempts_truncated, ...baseValue } = value;
  const base = webhookEndpoint(baseValue);
  if (!base || !Array.isArray(delivery_attempts) || delivery_attempts.length > 25 || typeof delivery_attempts_truncated !== "boolean") return { status: 503, body: { error: { code: "evidence_unavailable" } } };
  const attempts = delivery_attempts.map(webhookDeliveryAttempt);
  if (attempts.some((item) => item === null)) return { status: 503, body: { error: { code: "evidence_unavailable" } } };
  return { status: 200, body: { ...base, delivery_attempts: attempts, delivery_attempts_truncated } };
}

export function sanitizeOperationsBody(status: number, raw: string, sanitizer: (status: number, value: unknown) => SanitizedOperationsResponse): SanitizedOperationsResponse {
  if (new TextEncoder().encode(raw).byteLength > maximumUpstreamBytes) return { status: 503, body: { error: { code: "evidence_unavailable" } } };
  try { return sanitizer(status, JSON.parse(raw) as unknown); }
  catch { return { status: 503, body: { error: { code: "evidence_unavailable" } } }; }
}
