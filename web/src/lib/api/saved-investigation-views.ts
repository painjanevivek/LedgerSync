export const savedViewFilterSchemaVersion = "1" as const;
export const maximumSavedViews = 25;

export type SavedViewDomain = "accounts" | "transfers" | "funding" | "approvals" | "corrections" | "events" | "webhooks";
export type SavedInvestigationView = Readonly<{
  saved_view_id: string;
  name: string;
  filter_schema_version: typeof savedViewFilterSchemaVersion;
  domain: SavedViewDomain;
  filters: Readonly<Record<string, string>>;
  target_path: string;
  version: string;
  created_at: string;
  updated_at: string;
}>;
export type SavedInvestigationViewPage = Readonly<{ views: SavedInvestigationView[]; generated_at: string }>;
export type SavedViewCreateInput = Readonly<{ name: string; filter_schema_version: typeof savedViewFilterSchemaVersion; domain: SavedViewDomain; filters: Readonly<Record<string, string>> }>;
export type SavedViewRenameInput = Readonly<{ expected_version: string; name: string }>;

type SanitizedSavedViewResponse = Readonly<{ status: number; body: SavedInvestigationView | SavedInvestigationViewPage | Readonly<{ error: Readonly<{ code: string }> }> }>;
type Rule = (value: string) => string | null;

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/u;
const exactCount = /^(?:0|[1-9][0-9]{0,18})$/u;
const transferQuery = /^[0-9a-f-]{1,128}$/u;
const eventType = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/u;
const domains = new Set<SavedViewDomain>(["accounts", "transfers", "funding", "approvals", "corrections", "events", "webhooks"]);
const controlCharacters = /[\u0000-\u001f\u007f-\u009f]/u;

function record(value: unknown): Record<string, unknown> | null { return typeof value === "object" && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : null; }
function exactKeys(value: Record<string, unknown>, required: readonly string[], optional: readonly string[] = []) { const allowed = new Set([...required, ...optional]); return required.every((key) => Object.hasOwn(value, key)) && Object.keys(value).every((key) => allowed.has(key)); }
function timestamp(value: unknown): value is string { return typeof value === "string" && value.length <= 64 && /(?:Z|[+-][0-9]{2}:[0-9]{2})$/u.test(value) && !Number.isNaN(Date.parse(value)); }
function viewName(value: unknown): value is string { return typeof value === "string" && value === value.trim() && [...value].length >= 1 && [...value].length <= 80 && !controlCharacters.test(value); }
function oneOf(...values: string[]): Rule { const allowed = new Set(values); return (value) => allowed.has(value) ? value : null; }
function canonicalUUID(value: string) { const normalized = value.toLowerCase(); return uuid.test(normalized) ? normalized : null; }
function canonicalTimestamp(value: string) { if (value.length > 64 || !/(?:Z|[+-][0-9]{2}:[0-9]{2})$/u.test(value) || Number.isNaN(Date.parse(value))) return null; return new Date(value).toISOString().replace(".000Z", "Z"); }
function utcDate(value: string) { if (!/^\d{4}-\d{2}-\d{2}$/u.test(value)) return null; const parsed = new Date(`${value}T00:00:00Z`); return !Number.isNaN(parsed.valueOf()) && parsed.toISOString().slice(0, 10) === value ? value : null; }

function rulesFor(domain: SavedViewDomain): Readonly<{ path: string; rules: Readonly<Record<string, Rule>> }> {
  switch (domain) {
    case "accounts": return { path: "/accounts", rules: { status: oneOf("active", "frozen", "closed"), category: oneOf("operating", "customer_funds", "payroll", "payables", "expenses", "reserve") } };
    case "transfers": return { path: "/transfers", rules: { q: (value) => { const normalized = value.toLowerCase(); return transferQuery.test(normalized) ? normalized : null; }, accountId: canonicalUUID, status: oneOf("pending", "posted", "rejected"), from: canonicalTimestamp, to: canonicalTimestamp } };
    case "funding": return { path: "/funding", rules: { status: oneOf("requested", "approved", "posted", "rejected", "compensated") } };
    case "approvals": return { path: "/approvals", rules: { domain: oneOf("funding", "correction"), status: oneOf("funding:requested", "funding:approved", "funding:posted", "funding:rejected", "funding:compensated", "correction:requested", "correction:approved", "correction:rejected", "correction:cancelled", "correction:expired", "correction:posted"), age: oneOf("under_24h", "over_24h", "over_7d", "over_30d"), requested_after: utcDate, requested_before: utcDate, actionable_by_me: oneOf("true") } };
    case "corrections": return { path: "/corrections", rules: { status: oneOf("requested", "approved", "posted", "rejected", "cancelled", "expired") } };
    case "events": return { path: "/events", rules: { eventType: (value) => eventType.test(value) ? value : null, state: oneOf("pending", "retrying", "published", "dead"), endpointId: canonicalUUID, relatedId: canonicalUUID, correlationId: canonicalUUID, from: canonicalTimestamp, to: canonicalTimestamp } };
    case "webhooks": return { path: "/webhooks", rules: { status: oneOf("pending_verification", "active", "disabled"), eventType: (value) => eventType.test(value) ? value : null } };
  }
}

export function normalizeSavedViewDefinition(domain: SavedViewDomain, input: Readonly<Record<string, unknown>>): Readonly<{ filters: Readonly<Record<string, string>>; targetPath: string }> | null {
  const entries = Object.entries(input);
  if (!domains.has(domain) || entries.length < 1 || entries.length > 8) return null;
  const definition = rulesFor(domain);
  const filters: Record<string, string> = {};
  for (const [key, raw] of entries) {
    const rule = definition.rules[key];
    if (!rule || typeof raw !== "string" || raw === "" || raw !== raw.trim()) return null;
    const normalized = rule(raw);
    if (normalized === null) return null;
    filters[key] = normalized;
  }
  if (filters.from && filters.to && filters.from > filters.to) return null;
  if (filters.requested_after && filters.requested_before && filters.requested_after > filters.requested_before) return null;
  if (domain === "approvals" && filters.domain && filters.status && !filters.status.startsWith(`${filters.domain}:`)) return null;
  const query = new URLSearchParams();
  for (const key of Object.keys(filters).sort()) query.set(key, filters[key]);
  return { filters, targetPath: `${definition.path}?${query}` };
}

export function createSavedViewInput(name: string, domain: SavedViewDomain, rawFilters: Readonly<Record<string, unknown>>): SavedViewCreateInput {
  if (!viewName(name)) throw new Error("invalid saved view name");
  const definition = normalizeSavedViewDefinition(domain, rawFilters);
  if (!definition) throw new Error("invalid saved view filters");
  return { name, filter_schema_version: savedViewFilterSchemaVersion, domain, filters: definition.filters };
}

export function parseSavedViewCreateInput(value: unknown): SavedViewCreateInput {
  const root = record(value);
  if (!root || !exactKeys(root, ["name", "filter_schema_version", "domain", "filters"]) || !viewName(root.name) || root.filter_schema_version !== savedViewFilterSchemaVersion || typeof root.domain !== "string" || !domains.has(root.domain as SavedViewDomain)) throw new Error("invalid saved view input");
  const filters = record(root.filters);
  const definition = filters ? normalizeSavedViewDefinition(root.domain as SavedViewDomain, filters) : null;
  if (!definition) throw new Error("invalid saved view input");
  return { name: root.name, filter_schema_version: savedViewFilterSchemaVersion, domain: root.domain as SavedViewDomain, filters: definition.filters };
}

export function parseSavedViewRenameInput(value: unknown): SavedViewRenameInput {
  const root = record(value);
  if (!root || !exactKeys(root, ["expected_version", "name"]) || typeof root.expected_version !== "string" || !exactCount.test(root.expected_version) || root.expected_version === "0" || !viewName(root.name)) throw new Error("invalid saved view rename");
  return { expected_version: root.expected_version, name: root.name };
}

function sanitizeView(value: unknown): SavedInvestigationView | null {
  const item = record(value);
  if (!item || !exactKeys(item, ["saved_view_id", "name", "filter_schema_version", "domain", "filters", "target_path", "version", "created_at", "updated_at"]) || typeof item.saved_view_id !== "string" || !uuid.test(item.saved_view_id) || !viewName(item.name) || item.filter_schema_version !== savedViewFilterSchemaVersion || typeof item.domain !== "string" || !domains.has(item.domain as SavedViewDomain) || typeof item.target_path !== "string" || typeof item.version !== "string" || !exactCount.test(item.version) || item.version === "0" || !timestamp(item.created_at) || !timestamp(item.updated_at)) return null;
  const filters = record(item.filters);
  const definition = filters ? normalizeSavedViewDefinition(item.domain as SavedViewDomain, filters) : null;
  if (!definition || definition.targetPath !== item.target_path) return null;
  return { saved_view_id: item.saved_view_id, name: item.name, filter_schema_version: savedViewFilterSchemaVersion, domain: item.domain as SavedViewDomain, filters: definition.filters, target_path: definition.targetPath, version: item.version, created_at: item.created_at, updated_at: item.updated_at };
}

function sanitizedError(status: number, value: unknown): SanitizedSavedViewResponse {
  const root = record(value); const detail = record(root?.error); const code = detail?.code;
  const allowed = new Set(["unauthorized", "forbidden", "invalid_request", "validation_failed", "unsupported_media_type", "not_found", "saved_view_version_conflict", "saved_view_name_conflict", "saved_view_limit_reached", "rate_limited", "evidence_unavailable", "temporary_unavailable", "upstream_timeout"]);
  if (typeof code === "string" && allowed.has(code) && [400, 401, 403, 404, 409, 415, 429, 503, 504].includes(status)) return { status, body: { error: { code } } };
  return { status: 503, body: { error: { code: "evidence_unavailable" } } };
}

export function sanitizeSavedViewPage(status: number, value: unknown): SanitizedSavedViewResponse {
  if (status !== 200) return sanitizedError(status, value);
  const root = record(value);
  if (!root || !exactKeys(root, ["views", "generated_at"]) || !timestamp(root.generated_at) || !Array.isArray(root.views) || root.views.length > maximumSavedViews) return sanitizedError(503, value);
  const views = root.views.map(sanitizeView);
  if (views.some((view) => view === null) || new Set(views.map((view) => view?.saved_view_id)).size !== views.length) return sanitizedError(503, value);
  return { status: 200, body: { views: views as SavedInvestigationView[], generated_at: root.generated_at } };
}

export function sanitizeSavedView(status: number, value: unknown): SanitizedSavedViewResponse {
  if (status !== 200 && status !== 201) return sanitizedError(status, value);
  const view = sanitizeView(value);
  return view ? { status, body: view } : sanitizedError(503, value);
}

export async function readBoundedSavedViewBody(response: Response): Promise<unknown> {
  const declared = response.headers.get("content-length");
  if (declared && (!/^\d+$/u.test(declared) || Number(declared) > 65_536)) throw new Error("saved view response too large");
  const text = await response.text();
  if (new TextEncoder().encode(text).byteLength > 65_536) throw new Error("saved view response too large");
  return text ? JSON.parse(text) : {};
}
