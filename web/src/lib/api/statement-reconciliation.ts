export const maxStatementSourceBytes = 1048576;
export const maxStatementRequestBytes = 1500000;
export const statementUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const token = /^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$/;
const minor = /^(0|-?[1-9][0-9]{0,18})$/;
const positive = /^[1-9][0-9]{0,18}$/;
const version = /^(0|[1-9][0-9]{0,18})$/;
const record = (value: unknown): value is Record<string, unknown> => !!value && typeof value === "object" && !Array.isArray(value);
const bounded = (value: unknown, max = 256): value is string => typeof value === "string" && new TextEncoder().encode(value).byteLength <= max;
const exact = (value: unknown, signed = true): value is string => typeof value === "string" && (signed ? minor : version).test(value) && BigInt(value) >= (signed ? -9223372036854775808n : 0n) && BigInt(value) <= 9223372036854775807n;
const id = (value: unknown): value is string => typeof value === "string" && statementUUID.test(value) && value !== "00000000-0000-0000-0000-000000000000";
const date = (value: unknown): value is string => typeof value === "string" && value.length < 64 && Number.isFinite(Date.parse(value));
const count = (value: unknown, max = 3000): value is number => typeof value === "number" && Number.isSafeInteger(value) && value >= 0 && value <= max;
const hash = (value: unknown): value is string => typeof value === "string" && /^[0-9a-f]{64}$/.test(value);
export type StatementReceipt = Readonly<{
    id: string;
    replayed: boolean;
}>;
export type StatementPreview = Readonly<{
    sha256: string;
    source_account_ref: string;
    period_start: string;
    period_end: string;
    opening_minor: string;
    closing_minor: string;
    row_count: number;
}>;
export type StatementMapping = Readonly<{
    id: string;
    account_id: string;
    source_account_ref: string;
    currency: "INR";
    version: 1;
    created_by: string;
    created_at: string;
}>;
export type StatementImport = Readonly<{
    id: string;
    mapping_id: string;
    sha256: string;
    row_count: number;
    period_start: string;
    period_end: string;
    opening_minor: string;
    closing_minor: string;
    created_at: string;
    run_id?: string;
    run_state?: "prepared" | "completed";
}>;
export type StatementWorkspace = Readonly<{
    mapping: StatementMapping | null;
    imports: readonly StatementImport[];
    accounts: readonly Readonly<{
        id: string;
        display_name: string;
        kind: string;
    }>[];
    more_accounts: boolean;
}>;
export type StatementRun = Readonly<{
    id: string;
    import_id: string;
    mapping_id: string;
    account_id: string;
    source_account_ref: string;
    state: "prepared" | "completed";
    rule_version: 1;
    mapping_version: 1;
    cutoff: string;
    period_start: string;
    period_end: string;
    source_sha256: string;
    statement_row_count: number;
    snapshot_item_count: number;
    matched_row_count: number;
    unmatched_statement_count: number;
    unmatched_ledger_count: number;
}>;
export type StatementReview = Readonly<{
    kind: "statement" | "ledger";
    item_id: string;
    version: string;
    classification: string;
    status: "open" | "reviewed_exception" | "matched";
    external_reference: string;
    direction: "debit" | "credit";
    amount_minor: string;
    currency: "INR";
    assignee: string;
    reason: string;
    comment: string;
    evidence: string;
    linked_posting_id: string;
    updated_by: string;
    updated_at: string;
}>;
export type StatementRunPage = Readonly<{
    run: StatementRun;
    items: readonly StatementReview[];
    next_cursor?: string;
}>;
export type StatementReviewInput = Readonly<{
    kind: "statement" | "ledger";
    item_id: string;
    expected_version: string;
    status: "open" | "reviewed_exception" | "matched";
    assignee: string;
    reason: string;
    comment: string;
    evidence: string;
    posting_id?: string;
}>;
export type StatementBodyKind = "mapping" | "preview" | "import" | "run" | "review";
export function parseStatementBody(value: unknown, kind: StatementBodyKind): Record<string, unknown> {
    if (!record(value))
        throw new Error("invalid statement body");
    const fields: Record<StatementBodyKind, readonly string[]> = { mapping: ["account_id", "source_account_ref"], preview: ["source_base64"], import: ["mapping_id", "source_base64"], run: ["import_id"], review: ["kind", "item_id", "expected_version", "status", "assignee", "reason", "comment", "evidence", "posting_id"] };
    if (Object.keys(value).some(key => !fields[kind].includes(key)))
        throw new Error("unknown statement field");
    if (kind === "mapping" && (!id(value.account_id) || typeof value.source_account_ref !== "string" || !token.test(value.source_account_ref)))
        throw new Error("invalid statement mapping");
    if (kind === "run" && !id(value.import_id))
        throw new Error("invalid statement import");
    if (kind === "import" && !id(value.mapping_id))
        throw new Error("invalid mapping identity");
    if (kind === "preview" || kind === "import") {
        const source = value.source_base64;
        if (typeof source !== "string" || source.length < 4 || source.length > 1398104 || source.length % 4 || !/^[A-Za-z0-9+/]+={0,2}$/.test(source) || source.length / 4 * 3 - (source.endsWith("==") ? 2 : source.endsWith("=") ? 1 : 0) > maxStatementSourceBytes)
            throw new Error("invalid bounded source");
    }
    if (kind === "review") {
        if (!["statement", "ledger"].includes(String(value.kind)) || typeof value.item_id !== "string" || !token.test(value.item_id) || !exact(value.expected_version, false) || !["open", "reviewed_exception", "matched"].includes(String(value.status)) || !bounded(value.assignee) || /[\x00-\x1f\x7f]/.test(value.assignee) || !bounded(value.reason, 64) || !/^[a-z][a-z_]{0,63}$/.test(value.reason) || !bounded(value.comment, 512) || !bounded(value.evidence) || value.evidence !== "" && !/^\/(transfers|funding|reconciliation)\/[A-Za-z0-9/_-]+$/.test(value.evidence) || value.posting_id !== undefined && !id(value.posting_id) || value.status === "matched" && (value.kind !== "statement" || !id(value.posting_id)) || value.status !== "matched" && value.posting_id !== undefined)
            throw new Error("invalid statement review");
    }
    return value;
}
export const isStatementReceipt = (v: unknown): v is StatementReceipt => record(v) && id(v.id) && typeof v.replayed === "boolean";
export const isStatementPreview = (v: unknown): v is StatementPreview => record(v) && hash(v.sha256) && typeof v.source_account_ref === "string" && token.test(v.source_account_ref) && date(v.period_start) && date(v.period_end) && exact(v.opening_minor) && exact(v.closing_minor) && count(v.row_count, 1000) && v.row_count > 0;
function isMapping(v: unknown): v is StatementMapping { return record(v) && id(v.id) && id(v.account_id) && typeof v.source_account_ref === "string" && token.test(v.source_account_ref) && v.currency === "INR" && v.version === 1 && bounded(v.created_by) && date(v.created_at); }
function isImport(v: unknown): v is StatementImport { return record(v) && id(v.id) && id(v.mapping_id) && hash(v.sha256) && count(v.row_count, 1000) && v.row_count > 0 && date(v.period_start) && date(v.period_end) && exact(v.opening_minor) && exact(v.closing_minor) && date(v.created_at) && (v.run_id === undefined || id(v.run_id)) && (v.run_state === undefined || ["prepared", "completed"].includes(String(v.run_state))); }
export function isStatementWorkspace(v: unknown): v is StatementWorkspace { return record(v) && (v.mapping === null || isMapping(v.mapping)) && Array.isArray(v.imports) && v.imports.length <= 100 && v.imports.every(isImport) && Array.isArray(v.accounts) && v.accounts.length <= 100 && v.accounts.every(a => record(a) && id(a.id) && bounded(a.display_name, 512) && ["customer", "funding_clearing"].includes(String(a.kind))) && typeof v.more_accounts === "boolean"; }
function isRun(v: unknown): v is StatementRun { return record(v) && [v.id, v.import_id, v.mapping_id, v.account_id].every(id) && bounded(v.source_account_ref, 128) && ["prepared", "completed"].includes(String(v.state)) && v.rule_version === 1 && v.mapping_version === 1 && date(v.cutoff) && date(v.period_start) && date(v.period_end) && hash(v.source_sha256) && [v.statement_row_count, v.snapshot_item_count, v.matched_row_count, v.unmatched_statement_count, v.unmatched_ledger_count].every(n => count(n)); }
function isReview(v: unknown): v is StatementReview { return record(v) && ["statement", "ledger"].includes(String(v.kind)) && typeof v.item_id === "string" && token.test(v.item_id) && exact(v.version, false) && ["matched", "ambiguous", "unmatched_external", "unmatched_internal", "amount_or_direction_difference", "already_consumed"].includes(String(v.classification)) && ["open", "reviewed_exception", "matched"].includes(String(v.status)) && bounded(v.external_reference, 128) && ["debit", "credit"].includes(String(v.direction)) && typeof v.amount_minor === "string" && positive.test(v.amount_minor) && exact(v.amount_minor) && v.currency === "INR" && bounded(v.assignee) && bounded(v.reason, 64) && bounded(v.comment, 512) && bounded(v.evidence) && (v.linked_posting_id === "" || id(v.linked_posting_id)) && bounded(v.updated_by) && date(v.updated_at); }
export function isStatementRunPage(v: unknown): v is StatementRunPage { return record(v) && isRun(v.run) && Array.isArray(v.items) && v.items.length <= 100 && v.items.every(isReview) && (v.next_cursor === undefined || bounded(v.next_cursor, 512)); }
export type StatementRoute = Readonly<{
    path: string;
    write: boolean;
    export: boolean;
    body?: StatementBodyKind;
    result: "workspace" | "preview" | "receipt" | "page" | "csv";
    cursor?: boolean;
    source?: boolean;
    resourceID?: string;
}>;
export function statementRoute(method: string, parts: readonly string[]): StatementRoute | null {
    const path = "/api/statement-reconciliation" + (parts.length ? "/" + parts.map(encodeURIComponent).join("/") : "");
    if (method === "GET" && parts.length === 0)
        return { path, write: false, export: false, result: "workspace" };
    if (method === "POST" && parts.length === 1) {
        const kind = ({ mappings: "mapping", preview: "preview", imports: "import", runs: "run" } as Partial<Record<string, StatementBodyKind>>)[parts[0]];
        if (kind)
            return { path, write: true, export: false, body: kind, result: kind === "preview" ? "preview" : "receipt" };
    }
    if (parts.length >= 2 && id(parts[1])) {
        if (method === "GET" && parts[0] === "runs" && parts.length === 2)
            return { path, write: false, export: false, result: "page", cursor: true, resourceID: parts[1] };
        if (method === "POST" && parts[0] === "runs" && parts.length === 3 && parts[2] === "reviews")
            return { path, write: true, export: false, body: "review", result: "receipt", resourceID: parts[1] };
        if (method === "GET" && parts.length === 3 && (parts[0] === "runs" && parts[2] === "comparison.csv" || parts[0] === "imports" && parts[2] === "source.csv"))
            return { path, write: false, export: true, result: "csv", source: parts[0] === "imports", resourceID: parts[1] };
    }
    return null;
}
