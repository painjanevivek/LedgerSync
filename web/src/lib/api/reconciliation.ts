export const reconciliationMutationMaximumBytes = 256;

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const canonicalCount = /^(?:0|[1-9][0-9]*)$/;
const runStatuses = new Set(["matched", "mismatch", "failed", "running"]);
const reconciliationUpstreamMaximumBytes = 65_536;

export type ReconciliationRunRequest = Readonly<Record<string, never>>;

export type SanitizedReconciliationUpstream = Readonly<{
  status: number;
  body: Readonly<Record<string, unknown>>;
}>;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isBoundedString(value: unknown, maximumLength: number, allowEmpty = false): value is string {
  return typeof value === "string" && value.length <= maximumLength && (allowEmpty || value.length > 0);
}

function isDate(value: unknown, allowEmpty = false): value is string {
  return isBoundedString(value, 64, allowEmpty) && (value === "" ? allowEmpty : !Number.isNaN(Date.parse(value)));
}

function isMismatch(value: unknown): value is Record<string, unknown> {
  if (!isRecord(value)) return false;
  const allowed = new Set([
    "mismatch_id", "account_id", "classification", "currency", "expected_minor",
    "observed_minor", "observed_available_minor", "balance_version", "created_at",
  ]);
  if (Object.keys(value).some((key) => !allowed.has(key))) return false;
  return isBoundedString(value.mismatch_id, 128)
    && isBoundedString(value.classification, 128)
    && isDate(value.created_at)
    && (value.account_id === undefined || isBoundedString(value.account_id, 128))
    && (value.currency === undefined || value.currency === "INR")
    && [value.expected_minor, value.observed_minor, value.observed_available_minor, value.balance_version]
      .every((item) => item === undefined || isBoundedString(item, 64));
}

function sanitizeRun(value: unknown): Readonly<Record<string, unknown>> | null {
  if (!isRecord(value)
    || !isBoundedString(value.run_id, 128)
    || !uuid.test(value.run_id)
    || typeof value.status !== "string"
    || !runStatuses.has(value.status)
    || !isBoundedString(value.correlation_id, 128)
    || !isBoundedString(value.scope, 256)
    || !isBoundedString(value.ledger_watermark, 256, value.status === "running")
    || !isBoundedString(value.application_version, 128, value.status === "running")
    || !isBoundedString(value.schema_version, 128, value.status === "running")
    || typeof value.checked_account_count !== "string"
    || !canonicalCount.test(value.checked_account_count)
    || typeof value.posting_count !== "string"
    || !canonicalCount.test(value.posting_count)
    || typeof value.mismatch_count !== "string"
    || !canonicalCount.test(value.mismatch_count)
    || !isDate(value.started_at)
    || !isDate(value.completed_at, value.status === "running")
    || value.mismatches !== undefined && (!Array.isArray(value.mismatches) || value.mismatches.length > 500 || !value.mismatches.every(isMismatch))) {
    return null;
  }
  return {
    run_id: value.run_id,
    status: value.status,
    correlation_id: value.correlation_id,
    scope: value.scope,
    ledger_watermark: value.ledger_watermark,
    application_version: value.application_version,
    schema_version: value.schema_version,
    checked_account_count: value.checked_account_count,
    posting_count: value.posting_count,
    mismatch_count: value.mismatch_count,
    started_at: value.started_at,
    completed_at: value.completed_at,
    ...(value.mismatches === undefined ? {} : { mismatches: value.mismatches.map((item) => ({ ...item })) }),
  };
}

export function isValidReconciliationIdempotencyKey(value: string | null): value is string {
  if (value === null || value.length < 16 || value.length > 255) return false;
  for (const character of value) {
    const code = character.charCodeAt(0);
    if (code < 0x21 || code > 0x7e) return false;
  }
  return true;
}

export function parseReconciliationRunRequest(value: unknown): ReconciliationRunRequest {
  if (!isRecord(value) || Object.keys(value).length !== 0) throw new Error("reconciliation request must be an empty object");
  return {};
}

function typedError(status: number, value: unknown): SanitizedReconciliationUpstream {
  const error = isRecord(value) && isRecord(value.error) ? value.error : undefined;
  const code = typeof error?.code === "string" ? error.code : "";
  if (status === 409 && (code === "reconciliation_already_running" || code === "request_in_progress")) {
    const runID = isRecord(value) && isBoundedString(value.run_id, 128) && uuid.test(value.run_id) ? value.run_id : undefined;
    return { status, body: { error: { code }, ...(runID ? { run_id: runID } : {}) } };
  }
  if (status === 400 && (code === "validation_failed" || code === "idempotency_key_required")) return { status, body: { error: { code } } };
  if (status === 401 && code === "unauthorized") return { status, body: { error: { code } } };
  if (status === 403 && code === "forbidden") return { status, body: { error: { code } } };
  if (status === 409 && code === "idempotency_conflict") return { status, body: { error: { code } } };
  if (status === 429 && code === "rate_limited") return { status, body: { error: { code } } };
  if (status === 503 && code === "temporary_unavailable") return { status, body: { error: { code } } };
  if (status === 504 && (code === "response_unknown" || code === "reconciliation_outcome_unknown")) {
    return { status, body: { error: { code: "reconciliation_outcome_unknown" } } };
  }
  return { status: 503, body: { error: { code: "temporary_unavailable" } } };
}

function unknownOutcome(): SanitizedReconciliationUpstream {
  return { status: 504, body: { error: { code: "reconciliation_outcome_unknown" } } };
}

export function sanitizeReconciliationUpstream(status: number, value: unknown): SanitizedReconciliationUpstream {
  if (status >= 200 && status < 300) {
    const run = sanitizeRun(value);
    return run ? { status: status === 201 ? 201 : status === 202 ? 202 : 200, body: run } : unknownOutcome();
  }
  return typedError(status, value);
}

export function sanitizeReconciliationUpstreamBody(status: number, raw: string): SanitizedReconciliationUpstream {
  if (new TextEncoder().encode(raw).byteLength > reconciliationUpstreamMaximumBytes) {
    return status >= 200 && status < 300 ? unknownOutcome() : typedError(status, undefined);
  }
  try {
    return sanitizeReconciliationUpstream(status, JSON.parse(raw) as unknown);
  } catch {
    return status >= 200 && status < 300 ? unknownOutcome() : typedError(status, undefined);
  }
}
