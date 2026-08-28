export type DeveloperOperation = Readonly<{ operation_id: string; method: "GET" | "POST" | "PUT" | "PATCH"; path: string; scope: string }>;
export type DeveloperEndpointGroup = Readonly<{ id: string; label: string; operations: DeveloperOperation[] }>;
export type DeveloperExample = Readonly<{
  id: "create_transfer" | "create_account";
  title: string;
  operation_id: string;
  request_schema: string;
  method: "POST";
  path: string;
  headers: Readonly<Record<string, string>>;
  body: Readonly<Record<string, string>>;
  retry_summary: string;
  result_facts?: Readonly<{ currency:string; status:string; available_minor:string; ledger_minor:string }>;
}>;
export type DeveloperMetadata = Readonly<{
  schema_version: "1";
  contract_version: string;
  base_url: "/api";
  openapi_download_path: "/api/openapi.yaml";
  boundary: Readonly<{ title: string; summary: string; network: "loopback_only" }>;
  authentication: ReadonlyArray<Readonly<{ id: "browser_bff_session" | "private_api_development_token"; label: string; summary: string }>>;
  endpoint_groups: DeveloperEndpointGroup[];
  examples: DeveloperExample[];
  retry_outcomes: ReadonlyArray<Readonly<{ code: string; safe_action: string }>>;
  error_catalogue: ReadonlyArray<Readonly<{ code: string; meaning: string }>>;
  record_lookup: Readonly<{ summary: string; correlation_header: "X-Request-ID"; safe_fields: string[] }>;
}>;

export type SanitizedDeveloperResponse = Readonly<{ status: number; body: Readonly<Record<string, unknown>> }>;

const identifier = /^[A-Za-z][A-Za-z0-9._:-]{0,127}$/;
const pathPattern = /^\/[A-Za-z0-9{}._/-]{1,255}$/;
const versionPattern = /^[0-9]+\.[0-9]+\.[0-9]+$/;
const visibleKey = /^[\x21-\x7e]{16,255}$/;
const endpointGroupIDs = new Set(["accounts", "transfers", "reconciliation", "operations", "recovery_exports", "developer"]);
const scopes = new Set(["accounts:read", "accounts:write", "transactions:read", "transfers:read", "transfers:write", "reconciliation:read", "reconciliation:write", "local:read", "local:write", "events:read", "explainability:read", "developer:read", "recovery:read", "exports:read"]);
const retryCodes = new Set(["request_in_progress", "response_unknown", "temporary_unavailable", "transaction_conflict_retryable", "idempotency_conflict", "reconciliation_already_running"]);
const errorCodes = new Set(["invalid_request", "unauthorized", "forbidden", "not_found", "account_version_conflict", "invalid_account_transition", "account_not_zero", "external_reference_conflict", "transfer_policy_denied", "evidence_unavailable"]);
const lookupFields = new Set(["account_id", "transfer_id", "run_id", "event_id", "correlation_id"]);

function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function exactKeys(value: Record<string, unknown>, required: readonly string[], optional: readonly string[] = []) {
  const permitted = new Set([...required, ...optional]);
  return required.every((key) => key in value) && Object.keys(value).every((key) => permitted.has(key));
}
function bounded(value: unknown, maximum = 512): value is string { return typeof value === "string" && value.length > 0 && value.length <= maximum && !/[\u0000-\u001f\u007f-\u009f]/u.test(value); }
function code(value: unknown): value is string { return bounded(value, 128) && identifier.test(value); }
function exactStringMap(value: unknown, keys: readonly string[]): value is Record<string, string> {
  return isRecord(value) && exactKeys(value, keys) && keys.every((key) => bounded(value[key], 256));
}

function sanitizeOperation(value: unknown): DeveloperOperation | null {
  if (!isRecord(value) || !exactKeys(value, ["operation_id", "method", "path", "scope"])
    || !code(value.operation_id) || typeof value.method !== "string" || !["GET", "POST", "PUT", "PATCH"].includes(value.method)
    || typeof value.path !== "string" || !pathPattern.test(value.path) || value.path.includes("//")
    || typeof value.scope !== "string" || !scopes.has(value.scope)) return null;
  return { operation_id:value.operation_id,method:value.method as DeveloperOperation["method"],path:value.path,scope:value.scope };
}

function sanitizeGroup(value: unknown): DeveloperEndpointGroup | null {
  if (!isRecord(value) || !exactKeys(value, ["id", "label", "operations"]) || typeof value.id !== "string" || !endpointGroupIDs.has(value.id) || !bounded(value.label, 64) || !Array.isArray(value.operations) || value.operations.length < 1 || value.operations.length > 12) return null;
  const operations = value.operations.map(sanitizeOperation);
  if (operations.some((operation) => operation === null)) return null;
  return { id:value.id,label:value.label,operations:operations as DeveloperOperation[] };
}

function sanitizeExample(value: unknown): DeveloperExample | null {
  if (!isRecord(value) || !exactKeys(value, ["id", "title", "operation_id", "request_schema", "method", "path", "headers", "body", "retry_summary"], ["result_facts"])
    || value.id !== "create_transfer" && value.id !== "create_account" || !bounded(value.title, 128) || !code(value.operation_id)
    || typeof value.request_schema !== "string" || !/^#\/components\/schemas\/[A-Za-z][A-Za-z0-9]{0,63}$/.test(value.request_schema)
    || value.method !== "POST" || typeof value.path !== "string" || !pathPattern.test(value.path) || !bounded(value.retry_summary, 512)) return null;
  const headers = value.headers;
  if (!exactStringMap(headers, ["Content-Type", "Idempotency-Key"]) || headers["Content-Type"] !== "application/json" || !visibleKey.test(headers["Idempotency-Key"])) return null;
  const bodyKeys = value.id === "create_transfer" ? ["source_account_id", "destination_account_id", "amount", "currency"] : ["display_name", "external_reference", "category", "currency"];
  if (!exactStringMap(value.body, bodyKeys) || value.body.currency !== "INR") return null;
  if (value.id === "create_transfer" && (typeof value.body.amount !== "string" || !/^[0-9]+(?:\.[0-9]+)?$/.test(value.body.amount))) return null;
  const facts = value.result_facts;
  if (facts !== undefined && (!exactStringMap(facts, ["currency", "status", "available_minor", "ledger_minor"]) || facts.currency !== "INR" || facts.status !== "active" || facts.available_minor !== "0" || facts.ledger_minor !== "0")) return null;
  return { id:value.id,title:value.title,operation_id:value.operation_id,request_schema:value.request_schema,method:"POST",path:value.path,headers:{...headers},body:{...value.body},retry_summary:value.retry_summary,...(facts === undefined ? {} : {result_facts:{currency:facts.currency,status:facts.status,available_minor:facts.available_minor,ledger_minor:facts.ledger_minor}}) };
}

function sanitizeMetadata(value: unknown): DeveloperMetadata | null {
  if (!isRecord(value) || !exactKeys(value, ["schema_version", "contract_version", "base_url", "openapi_download_path", "boundary", "authentication", "endpoint_groups", "examples", "retry_outcomes", "error_catalogue", "record_lookup"])
    || value.schema_version !== "1" || typeof value.contract_version !== "string" || !versionPattern.test(value.contract_version)
    || value.base_url !== "/api" || value.openapi_download_path !== "/api/openapi.yaml") return null;
  if (!isRecord(value.boundary) || !exactKeys(value.boundary, ["title", "summary", "network"]) || !bounded(value.boundary.title, 128) || !bounded(value.boundary.summary, 512) || value.boundary.network !== "loopback_only") return null;
  if (!Array.isArray(value.authentication) || value.authentication.length !== 2) return null;
  const authentication = value.authentication.map((entry) => isRecord(entry) && exactKeys(entry, ["id", "label", "summary"]) && (entry.id === "browser_bff_session" || entry.id === "private_api_development_token") && bounded(entry.label, 64) && bounded(entry.summary, 512) ? { id:entry.id,label:entry.label,summary:entry.summary } : null);
  if (authentication.some((entry) => entry === null) || new Set(authentication.map((entry) => entry?.id)).size !== 2) return null;
  if (!Array.isArray(value.endpoint_groups) || value.endpoint_groups.length !== endpointGroupIDs.size) return null;
  const groups = value.endpoint_groups.map(sanitizeGroup);
  if (groups.some((group) => group === null) || new Set(groups.map((group) => group?.id)).size !== endpointGroupIDs.size) return null;
  if (!Array.isArray(value.examples) || value.examples.length !== 2) return null;
  const examples = value.examples.map(sanitizeExample);
  if (examples.some((example) => example === null) || new Set(examples.map((example) => example?.id)).size !== 2) return null;
  if (!Array.isArray(value.retry_outcomes) || value.retry_outcomes.length !== retryCodes.size) return null;
  const retries = value.retry_outcomes.map((entry) => isRecord(entry) && exactKeys(entry, ["code", "safe_action"]) && typeof entry.code === "string" && retryCodes.has(entry.code) && bounded(entry.safe_action, 512) ? {code:entry.code,safe_action:entry.safe_action} : null);
  if (retries.some((entry) => entry === null) || new Set(retries.map((entry) => entry?.code)).size !== retryCodes.size) return null;
  if (!Array.isArray(value.error_catalogue) || value.error_catalogue.length !== errorCodes.size) return null;
  const errors = value.error_catalogue.map((entry) => isRecord(entry) && exactKeys(entry, ["code", "meaning"]) && typeof entry.code === "string" && errorCodes.has(entry.code) && bounded(entry.meaning, 512) ? {code:entry.code,meaning:entry.meaning} : null);
  if (errors.some((entry) => entry === null) || new Set(errors.map((entry) => entry?.code)).size !== errorCodes.size) return null;
  const lookup = value.record_lookup;
  if (!isRecord(lookup) || !exactKeys(lookup, ["summary", "correlation_header", "safe_fields"]) || !bounded(lookup.summary, 512) || lookup.correlation_header !== "X-Request-ID" || !Array.isArray(lookup.safe_fields) || lookup.safe_fields.length !== lookupFields.size || !lookup.safe_fields.every((field) => typeof field === "string" && lookupFields.has(field))) return null;
  return { schema_version:"1",contract_version:value.contract_version,base_url:"/api",openapi_download_path:"/api/openapi.yaml",boundary:{title:value.boundary.title,summary:value.boundary.summary,network:"loopback_only"},authentication:authentication as DeveloperMetadata["authentication"],endpoint_groups:groups as DeveloperEndpointGroup[],examples:examples as DeveloperExample[],retry_outcomes:retries as DeveloperMetadata["retry_outcomes"],error_catalogue:errors as DeveloperMetadata["error_catalogue"],record_lookup:{summary:lookup.summary,correlation_header:"X-Request-ID",safe_fields:[...lookup.safe_fields] as string[]} };
}

function sanitizedError(status: number, value: unknown): SanitizedDeveloperResponse {
  const error = isRecord(value) && isRecord(value.error) ? value.error : undefined;
  const expected = status === 401 ? "unauthorized" : status === 403 ? "forbidden" : status === 429 ? "rate_limited" : status === 503 ? "temporary_unavailable" : "";
  return expected && error?.code === expected ? {status,body:{error:{code:expected}}} : {status:503,body:{error:{code:"developer_contract_unavailable"}}};
}

export function sanitizeDeveloperMetadata(status: number, value: unknown): SanitizedDeveloperResponse {
  if (status < 200 || status >= 300) return sanitizedError(status, value);
  const metadata = sanitizeMetadata(value);
  return metadata ? {status:200,body:metadata} : {status:503,body:{error:{code:"developer_contract_unavailable"}}};
}
