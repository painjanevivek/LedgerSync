import { sanitizeInvestigationSearch, type InvestigationSearchResult } from "@/lib/api/investigation-search";
import { sanitizeRelatedEvidence, type RelatedEvidence } from "@/lib/api/related-evidence";

export const workspaceTaxonomies = ["account_state", "transfer_delivery", "funding", "reconciliation", "correction", "other"] as const;
export const workspaceRecordTypes = ["account", "transfer", "funding", "event", "reconciliation_run", "reconciliation_mismatch", "correction"] as const;
export type WorkspaceTaxonomy = typeof workspaceTaxonomies[number];
export type WorkspaceRecordType = typeof workspaceRecordTypes[number];

export type WorkspaceSummary = Readonly<{ investigation_id: string; title: string; taxonomy: WorkspaceTaxonomy; status: "open" | "closed"; version: string; created_at: string; updated_at: string; closed_at?: string }>;
export type WorkspacePage = Readonly<{ investigations: WorkspaceSummary[]; generated_at: string }>;
export type WorkspaceQueryContext = Readonly<{ kind: "immutable_id" | "approved_reference"; record_type: WorkspaceRecordType; value: string }>;
export type WorkspaceReference = Readonly<{ relationship_type: string; source_record_type?: WorkspaceRecordType; source_record_id?: string; record_type: WorkspaceRecordType; record_id: string; target_path: string; captured_at: string }>;
export type WorkspaceHistoryItem = Readonly<{ action: "created" | "handed_off" | "closed" | "reopened"; actor_is_current_operator: boolean; version: string; status: "open" | "closed"; occurred_at: string }>;
export type InvestigationWorkspace = WorkspaceSummary & Readonly<{
  historical_context: Readonly<{ query_context: WorkspaceQueryContext; references: WorkspaceReference[]; withheld_reference_count: number; history: WorkspaceHistoryItem[]; history_truncated: boolean }>;
  current_evidence: Readonly<{ root?: InvestigationSearchResult; relationships: RelatedEvidence[]; generated_at: string; truncated: boolean; available: boolean }>;
}>;
export type WorkspaceReceipt = Readonly<{ investigation_id: string; outcome: "handed_off" | "closed" | "reopened"; version: string; occurred_at: string }>;
export type WorkspaceCreateInput = Readonly<{ title: string; taxonomy: WorkspaceTaxonomy; query_context: WorkspaceQueryContext; root_record: Readonly<{ record_type: WorkspaceRecordType; record_id: string }> }>;

type SanitizedWorkspaceResponse = Readonly<{ status: number; body: WorkspacePage | InvestigationWorkspace | WorkspaceReceipt | Readonly<{ error: Readonly<{ code: string }> }> }>;
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/u;
const exactCount = /^(?:0|[1-9][0-9]{0,18})$/u;
const approvedReference = /^[A-Za-z0-9][A-Za-z0-9._:/-]{7,127}$/u;
const relationship = /^[a-z][a-z0-9_]{1,63}$/u;
const subject = /^[^\u0000-\u001f\u007f-\u009f]{1,255}$/u;
const disallowedTitle = /(?:https?:\/\/|bearer\s|password|secret|token[=:]|api[_ -]?key|@)/iu;
const taxonomySet = new Set<string>(workspaceTaxonomies);
const recordTypeSet = new Set<string>(workspaceRecordTypes);
const controlCharacters = /[\u0000-\u001f\u007f-\u009f]/u;

function record(value: unknown): Record<string, unknown> | null { return typeof value === "object" && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : null; }
function exactKeys(value: Record<string, unknown>, required: readonly string[], optional: readonly string[] = []) { const allowed = new Set([...required, ...optional]); return required.every((key) => Object.hasOwn(value, key)) && Object.keys(value).every((key) => allowed.has(key)); }
function timestamp(value: unknown): value is string { return typeof value === "string" && value.length <= 64 && /(?:Z|[+-][0-9]{2}:[0-9]{2})$/u.test(value) && !Number.isNaN(Date.parse(value)); }
function safeTitle(value: unknown): value is string { return typeof value === "string" && value === value.trim() && [...value].length >= 1 && [...value].length <= 80 && !controlCharacters.test(value) && !/[<>]/u.test(value) && !disallowedTitle.test(value); }
function isTaxonomy(value: unknown): value is WorkspaceTaxonomy { return typeof value === "string" && taxonomySet.has(value); }
function isRecordType(value: unknown): value is WorkspaceRecordType { return typeof value === "string" && recordTypeSet.has(value); }
function isVersion(value: unknown): value is string { return typeof value === "string" && exactCount.test(value) && value !== "0"; }

export function parseWorkspaceCreateInput(value: unknown): WorkspaceCreateInput {
  const root = record(value); const query = record(root?.query_context); const rootRecord = record(root?.root_record);
  if (!root || !exactKeys(root, ["title", "taxonomy", "query_context", "root_record"]) || !safeTitle(root.title) || !isTaxonomy(root.taxonomy)
    || !query || !exactKeys(query, ["kind", "record_type", "value"]) || !isRecordType(query.record_type) || query.kind !== "immutable_id" && query.kind !== "approved_reference" || typeof query.value !== "string" || query.value !== query.value.trim()
    || !rootRecord || !exactKeys(rootRecord, ["record_type", "record_id"]) || !isRecordType(rootRecord.record_type) || rootRecord.record_type !== query.record_type || typeof rootRecord.record_id !== "string" || !uuid.test(rootRecord.record_id.toLowerCase())) throw new Error("invalid investigation workspace");
  const recordID = rootRecord.record_id.toLowerCase();
  const queryValue = query.kind === "immutable_id" ? query.value.toLowerCase() : query.value;
  if (query.kind === "immutable_id" ? queryValue !== recordID : !approvedReference.test(queryValue)) throw new Error("invalid investigation query context");
  return { title: root.title, taxonomy: root.taxonomy, query_context: { kind: query.kind, record_type: query.record_type, value: queryValue }, root_record: { record_type: rootRecord.record_type, record_id: recordID } };
}

export function parseWorkspaceHandoffInput(value: unknown): Readonly<{ expected_version: string; target_subject_id: string }> {
  const root = record(value);
  if (!root || !exactKeys(root, ["expected_version", "target_subject_id"]) || !isVersion(root.expected_version) || typeof root.target_subject_id !== "string" || root.target_subject_id !== root.target_subject_id.trim() || !subject.test(root.target_subject_id)) throw new Error("invalid investigation handoff");
  return { expected_version: root.expected_version, target_subject_id: root.target_subject_id };
}

export function parseWorkspaceStatusInput(value: unknown): Readonly<{ expected_version: string }> {
  const root = record(value);
  if (!root || !exactKeys(root, ["expected_version"]) || !isVersion(root.expected_version)) throw new Error("invalid investigation status change");
  return { expected_version: root.expected_version };
}

function sanitizeSummary(value: unknown): WorkspaceSummary | null {
  const item = record(value);
  if (!item || !exactKeys(item, ["investigation_id", "title", "taxonomy", "status", "version", "created_at", "updated_at"], ["closed_at"]) || typeof item.investigation_id !== "string" || !uuid.test(item.investigation_id) || !safeTitle(item.title) || !isTaxonomy(item.taxonomy) || item.status !== "open" && item.status !== "closed" || !isVersion(item.version) || !timestamp(item.created_at) || !timestamp(item.updated_at) || item.closed_at !== undefined && !timestamp(item.closed_at) || item.status === "open" && item.closed_at !== undefined || item.status === "closed" && item.closed_at === undefined) return null;
  return item as WorkspaceSummary;
}

function targetPath(type: WorkspaceRecordType, id: string) {
  switch (type) { case "account": return `/accounts/${id}`; case "transfer": return `/transfers/${id}`; case "funding": return `/funding/${id}`; case "event": return `/events/${id}`; case "reconciliation_run": return `/reconciliation/${id}`; case "correction": return `/corrections/${id}`; default: return ""; }
}

function sanitizeReference(value: unknown): WorkspaceReference | null {
  const item = record(value);
  if (!item || !exactKeys(item, ["relationship_type", "record_type", "record_id", "target_path", "captured_at"], ["source_record_type", "source_record_id"]) || typeof item.relationship_type !== "string" || !relationship.test(item.relationship_type) || !isRecordType(item.record_type) || typeof item.record_id !== "string" || !uuid.test(item.record_id) || typeof item.target_path !== "string" || item.target_path !== targetPath(item.record_type, item.record_id) || !timestamp(item.captured_at)) return null;
  const hasSourceType = item.source_record_type !== undefined; const hasSourceID = item.source_record_id !== undefined;
  if (hasSourceType !== hasSourceID || item.relationship_type === "root" && hasSourceType || item.relationship_type !== "root" && (!isRecordType(item.source_record_type) || typeof item.source_record_id !== "string" || !uuid.test(item.source_record_id))) return null;
  return item as WorkspaceReference;
}

function sanitizeHistory(value: unknown): WorkspaceHistoryItem | null {
  const item = record(value); const actions = new Set(["created", "handed_off", "closed", "reopened"]);
  if (!item || !exactKeys(item, ["action", "actor_is_current_operator", "version", "status", "occurred_at"]) || typeof item.action !== "string" || !actions.has(item.action) || typeof item.actor_is_current_operator !== "boolean" || !isVersion(item.version) || item.status !== "open" && item.status !== "closed" || !timestamp(item.occurred_at)) return null;
  return item as WorkspaceHistoryItem;
}

function sanitizedError(status: number, value: unknown): SanitizedWorkspaceResponse {
  const root = record(value); const detail = record(root?.error); const code = detail?.code;
  const allowed = new Set(["unauthorized", "forbidden", "csrf_failed", "invalid_request", "validation_failed", "unsupported_media_type", "not_found", "investigation_version_conflict", "investigation_state_conflict", "investigation_limit_reached", "rate_limited", "evidence_unavailable", "temporary_unavailable", "upstream_timeout"]);
  if (typeof code === "string" && allowed.has(code) && [400, 401, 403, 404, 409, 415, 429, 503, 504].includes(status)) return { status, body: { error: { code } } };
  return { status: 503, body: { error: { code: "evidence_unavailable" } } };
}

export function sanitizeWorkspacePage(status: number, value: unknown): SanitizedWorkspaceResponse {
  if (status !== 200) return sanitizedError(status, value);
  const root = record(value);
  if (!root || !exactKeys(root, ["investigations", "generated_at"]) || !Array.isArray(root.investigations) || root.investigations.length > 50 || !timestamp(root.generated_at)) return sanitizedError(503, value);
  const investigations = root.investigations.map(sanitizeSummary);
  if (investigations.some((item) => item === null) || new Set(investigations.map((item) => item?.investigation_id)).size !== investigations.length) return sanitizedError(503, value);
  return { status: 200, body: { investigations: investigations as WorkspaceSummary[], generated_at: root.generated_at } };
}

export function sanitizeWorkspace(status: number, value: unknown): SanitizedWorkspaceResponse {
  if (status !== 200 && status !== 201) return sanitizedError(status, value);
  const root = record(value); const summary = root ? sanitizeSummary({ investigation_id: root.investigation_id, title: root.title, taxonomy: root.taxonomy, status: root.status, version: root.version, created_at: root.created_at, updated_at: root.updated_at, ...(root.closed_at === undefined ? {} : { closed_at: root.closed_at }) }) : null; const historical = record(root?.historical_context); const query = record(historical?.query_context); const current = record(root?.current_evidence);
  if (!root || !summary || !exactKeys(root, ["investigation_id", "title", "taxonomy", "status", "version", "created_at", "updated_at", "historical_context", "current_evidence"], ["closed_at"])
    || !historical || !exactKeys(historical, ["query_context", "references", "withheld_reference_count", "history", "history_truncated"]) || !query || !exactKeys(query, ["kind", "record_type", "value"]) || query.kind !== "immutable_id" && query.kind !== "approved_reference" || !isRecordType(query.record_type) || typeof query.value !== "string" || query.value !== query.value.trim() || query.value.length < 8 || query.value.length > 128
    || !Array.isArray(historical.references) || historical.references.length > 21 || !Number.isInteger(historical.withheld_reference_count) || (historical.withheld_reference_count as number) < 0 || (historical.withheld_reference_count as number) > 21 || !Array.isArray(historical.history) || historical.history.length > 100 || typeof historical.history_truncated !== "boolean"
    || !current || !exactKeys(current, ["relationships", "generated_at", "truncated", "available"], ["root"]) || !Array.isArray(current.relationships) || current.relationships.length > 20 || !timestamp(current.generated_at) || typeof current.truncated !== "boolean" || typeof current.available !== "boolean") return sanitizedError(503, value);
  const references = historical.references.map(sanitizeReference); const history = historical.history.map(sanitizeHistory);
  if (references.some((item) => item === null) || history.some((item) => item === null) || new Set(references.map((item) => `${item?.record_type}:${item?.record_id}`)).size !== references.length) return sanitizedError(503, value);
  const currentSearch = sanitizeInvestigationSearch(200, { results: current.root ? [current.root] : [], query_kind: "immutable_id", generated_at: current.generated_at, truncated: false });
  const currentRelated = sanitizeRelatedEvidence(200, { source_type: query.record_type, source_id: references.find((item) => item?.relationship_type === "root")?.record_id ?? "", relationships: current.relationships, generated_at: current.generated_at, truncated: current.truncated });
  if (currentSearch.status !== 200 || currentRelated.status !== 200 || current.available && !current.root || !current.available && current.root !== undefined) return sanitizedError(503, value);
  return { status, body: { ...summary, historical_context: { query_context: query as WorkspaceQueryContext, references: references as WorkspaceReference[], withheld_reference_count: historical.withheld_reference_count as number, history: history as WorkspaceHistoryItem[], history_truncated: historical.history_truncated }, current_evidence: { root: current.root as InvestigationSearchResult | undefined, relationships: current.relationships as RelatedEvidence[], generated_at: current.generated_at, truncated: current.truncated, available: current.available } } };
}

export function sanitizeWorkspaceReceipt(status: number, value: unknown): SanitizedWorkspaceResponse {
  if (status !== 200) return sanitizedError(status, value);
  const root = record(value);
  if (!root || !exactKeys(root, ["investigation_id", "outcome", "version", "occurred_at"]) || typeof root.investigation_id !== "string" || !uuid.test(root.investigation_id) || root.outcome !== "handed_off" && root.outcome !== "closed" && root.outcome !== "reopened" || !isVersion(root.version) || !timestamp(root.occurred_at)) return sanitizedError(503, value);
  return { status, body: root as WorkspaceReceipt };
}

export async function readBoundedWorkspaceBody(response: Response): Promise<unknown> {
  const declared = response.headers.get("content-length");
  if (declared && (!/^\d+$/u.test(declared) || Number(declared) > 262_144)) throw new Error("investigation workspace response too large");
  const text = await response.text();
  if (new TextEncoder().encode(text).byteLength > 262_144) throw new Error("investigation workspace response too large");
  return text ? JSON.parse(text) : {};
}
