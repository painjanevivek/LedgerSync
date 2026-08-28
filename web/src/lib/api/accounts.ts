export const accountMutationMaximumBytes = 4_096;

const categories = new Set(["operating", "customer_funds", "payroll", "payables", "expenses", "reserve"]);
const statuses = new Set(["active", "frozen", "closed"]);
const canonicalPositiveInteger = /^[1-9][0-9]*$/;
const canonicalInteger = /^(?:0|-?[1-9][0-9]*)$/;
const externalReference = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const maximumSignedInteger = 9_223_372_036_854_775_807n;

export type CreateAccountRequest = Readonly<{
  display_name: string;
  external_reference: string;
  category: string;
  currency: "INR";
}>;

export type UpdateAccountRequest = Readonly<{
  expected_version: string;
  display_name: string;
  external_reference: string;
  category: string;
}> | Readonly<{
  expected_version: string;
  target_status: "active" | "frozen" | "closed";
  reason: string;
}>;

export type SanitizedAccountUpstream = Readonly<{
  status: number;
  body: Readonly<Record<string, unknown>>;
}>;

const accountUpstreamMaximumBytes = 65_536;

const publicErrorCodesByStatus: Readonly<Record<number, ReadonlySet<string>>> = {
  400: new Set(["invalid_request", "validation_failed"]),
  401: new Set(["unauthorized"]),
  403: new Set(["forbidden"]),
  404: new Set(["not_found"]),
  409: new Set(["account_version_conflict", "external_reference_conflict", "idempotency_conflict", "invalid_account_transition", "request_in_progress"]),
  422: new Set(["account_not_zero"]),
  429: new Set(["rate_limited"]),
  503: new Set(["temporary_unavailable"]),
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function hasExactlyKeys(value: Record<string, unknown>, expected: readonly string[]): boolean {
  const keys = Object.keys(value);
  return keys.length === expected.length && expected.every((key) => Object.hasOwn(value, key));
}

function isBoundedString(value: unknown, maximumLength: number, minimumLength = 1): value is string {
  if (typeof value !== "string") return false;
  const length = [...value].length;
  return length >= minimumLength && length <= maximumLength;
}

function isDisplayName(value: unknown): value is string {
  return isBoundedString(value, 120) && ![...value].some((character) => {
    const code = character.codePointAt(0) ?? 0;
    return code <= 0x1f || code >= 0x7f && code <= 0x9f;
  });
}

function isLifecycleReason(value: unknown): value is string {
  return isBoundedString(value, 256)
    && value.trim().length > 0
    && ![...value].some((character) => {
      const code = character.codePointAt(0) ?? 0;
      return code <= 0x1f || code >= 0x7f && code <= 0x9f;
    });
}

function isExternalReference(value: unknown): value is string {
  return typeof value === "string" && value.length >= 3 && value.length <= 64 && externalReference.test(value);
}

function isCanonicalPositiveVersion(value: unknown): value is string {
  if (typeof value !== "string" || value.length > 19 || !canonicalPositiveInteger.test(value)) return false;
  return BigInt(value) <= maximumSignedInteger;
}

export function isValidAccountIdempotencyKey(value: string | null): value is string {
  if (value === null || value.length < 16 || value.length > 255) return false;
  for (const character of value) {
    const code = character.charCodeAt(0);
    if (code < 0x21 || code > 0x7e) return false;
  }
  return true;
}

export function parseCreateAccountRequest(value: unknown): CreateAccountRequest {
  if (!isRecord(value) || !hasExactlyKeys(value, ["display_name", "external_reference", "category", "currency"])) {
    throw new Error("invalid account creation schema");
  }
  if (
    !isDisplayName(value.display_name)
    || !isExternalReference(value.external_reference)
    || typeof value.category !== "string"
    || !categories.has(value.category)
    || value.currency !== "INR"
  ) {
    throw new Error("invalid account creation fields");
  }
  return {
    display_name: value.display_name,
    external_reference: value.external_reference,
    category: value.category,
    currency: value.currency,
  };
}

export function parseUpdateAccountRequest(value: unknown): UpdateAccountRequest {
  if (!isRecord(value) || !isCanonicalPositiveVersion(value.expected_version)) {
    throw new Error("invalid account update schema");
  }
  if (hasExactlyKeys(value, ["expected_version", "display_name", "external_reference", "category"])) {
    if (
      !isDisplayName(value.display_name)
      || !isExternalReference(value.external_reference)
      || typeof value.category !== "string"
      || !categories.has(value.category)
    ) {
      throw new Error("invalid account metadata fields");
    }
    return {
      expected_version: value.expected_version,
      display_name: value.display_name,
      external_reference: value.external_reference,
      category: value.category,
    };
  }
  if (
    hasExactlyKeys(value, ["expected_version", "target_status", "reason"])
    && typeof value.target_status === "string"
    && statuses.has(value.target_status)
    && isLifecycleReason(value.reason)
  ) {
    return { expected_version: value.expected_version, target_status: value.target_status as "active" | "frozen" | "closed", reason: value.reason };
  }
  throw new Error("account update must contain exactly one supported command shape");
}

function defaultError(status: number): Readonly<{ status: number; code: string }> {
  switch (status) {
    case 400: return { status, code: "validation_failed" };
    case 401: return { status, code: "unauthorized" };
    case 403: return { status, code: "forbidden" };
    case 404: return { status, code: "not_found" };
    case 422: return { status, code: "account_not_zero" };
    case 429: return { status, code: "rate_limited" };
    default: return { status: 503, code: "temporary_unavailable" };
  }
}

function sanitizeError(status: number, value: unknown): SanitizedAccountUpstream {
  const error = isRecord(value) && isRecord(value.error) ? value.error : undefined;
  if (typeof error?.code === "string" && publicErrorCodesByStatus[status]?.has(error.code)) {
    return { status, body: { error: { code: error.code } } };
  }
  const fallback = defaultError(status);
  return { status: fallback.status, body: { error: { code: fallback.code } } };
}

function unknownSuccessfulOutcome(): SanitizedAccountUpstream {
  return { status: 504, body: { error: { code: "account_command_outcome_unknown" } } };
}

function sanitizeSuccess(status: number, value: unknown): SanitizedAccountUpstream {
  if (!isRecord(value)
    || typeof value.account_id !== "string"
    || !uuid.test(value.account_id)
    || typeof value.tenant_id !== "string"
    || !uuid.test(value.tenant_id)
    || value.currency !== "INR"
    || !isCanonicalPositiveVersion(value.account_version)
    || typeof value.status !== "string"
    || !statuses.has(value.status)
    || !isDisplayName(value.display_name)
    || !isExternalReference(value.external_reference)
    || typeof value.category !== "string"
    || !categories.has(value.category)
    || typeof value.available_minor !== "string"
    || value.available_minor.length > 20
    || !canonicalInteger.test(value.available_minor)
    || typeof value.ledger_minor !== "string"
    || value.ledger_minor.length > 20
    || !canonicalInteger.test(value.ledger_minor)
    || !isBoundedString(value.created_at, 64)
    || Number.isNaN(Date.parse(value.created_at))
    || !isBoundedString(value.updated_at, 64)
    || Number.isNaN(Date.parse(value.updated_at))) {
    return unknownSuccessfulOutcome();
  }
  return {
    status: status === 201 ? 201 : 200,
    body: {
      account_id: value.account_id,
      tenant_id: value.tenant_id,
      currency: value.currency,
      status: value.status,
      display_name: value.display_name,
      external_reference: value.external_reference,
      category: value.category,
      account_version: value.account_version,
      available_minor: value.available_minor,
      ledger_minor: value.ledger_minor,
      created_at: value.created_at,
      updated_at: value.updated_at,
    },
  };
}

export function sanitizeAccountUpstream(status: number, value: unknown): SanitizedAccountUpstream {
  return status >= 200 && status < 300 ? sanitizeSuccess(status, value) : sanitizeError(status, value);
}

export function sanitizeUnusableAccountUpstream(status: number): SanitizedAccountUpstream {
  // An unusable 2xx body can be the only surviving evidence of a durable
  // commit, so it is an unknown outcome. A non-2xx status does not assert
  // success: valid typed 4xx/422/503 codes remain authoritative, while an
  // unusable 5xx is reduced to generic temporary_unavailable. In both 503
  // cases callers retry the identical body and idempotency key.
  return status >= 200 && status < 300 ? unknownSuccessfulOutcome() : sanitizeError(status, undefined);
}

export function sanitizeAccountUpstreamBody(status: number, raw: string): SanitizedAccountUpstream {
  if (new TextEncoder().encode(raw).byteLength > accountUpstreamMaximumBytes) return sanitizeUnusableAccountUpstream(status);
  try {
    return sanitizeAccountUpstream(status, JSON.parse(raw) as unknown);
  } catch {
    return sanitizeUnusableAccountUpstream(status);
  }
}
