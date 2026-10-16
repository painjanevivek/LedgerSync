import { maxStatementSourceBytes, parseStatementBody, statementRoute, statementUUID, type StatementBodyKind } from "@/lib/api/statement-reconciliation";
export type StatementIntent = Readonly<{
    version: 1;
    tenantId: string;
    actorId: string;
    path: string;
    kind: StatementBodyKind;
    key: string;
    payload: string;
    sourceSHA256?: string;
    integrity: string;
}>;
export const statementStorageKey = (tenant: string, actor: string) => `ledgersync:statement-command:v1:${encodeURIComponent(tenant)}:${encodeURIComponent(actor)}`;
export async function statementDigest(bytes: Uint8Array): Promise<string> { return Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", bytes as BufferSource)), b => b.toString(16).padStart(2, "0")).join(""); }
const integrity = (intent: Omit<StatementIntent, "integrity">) => statementDigest(new TextEncoder().encode(JSON.stringify([intent.version, intent.tenantId, intent.actorId, intent.path, intent.kind, intent.key, intent.payload, intent.sourceSHA256 ?? null])));
function routeParts(path: string): string[] {
    if (!path.startsWith("/api/statement-reconciliation/"))
        throw new Error("Invalid statement destination.");
    return path.slice("/api/statement-reconciliation/".length).split("/");
}
export async function createStatementIntent(tenantId: string, actorId: string, path: string, kind: StatementBodyKind, body: unknown, sourceSHA256?: string): Promise<StatementIntent> {
    const route = statementRoute("POST", routeParts(path));
    if (!route || route.body !== kind || kind === "preview")
        throw new Error("Invalid statement command.");
    let payload: string;
    if (kind === "import") {
        const raw = body as {
            mapping_id?: unknown;
        };
        if (typeof raw?.mapping_id !== "string" || !statementUUID.test(raw.mapping_id) || !sourceSHA256 || !/^[0-9a-f]{64}$/.test(sourceSHA256))
            throw new Error("Original file digest is required.");
        payload = JSON.stringify({ mapping_id: raw.mapping_id, source_sha256: sourceSHA256 });
    }
    else
        payload = JSON.stringify(parseStatementBody(body, kind));
    const candidate = { version: 1 as const, tenantId, actorId, path, kind, key: crypto.randomUUID(), payload, ...(sourceSHA256 ? { sourceSHA256 } : {}) };
    return { ...candidate, integrity: await integrity(candidate) };
}
export async function parseStatementIntent(raw: string | null, tenantId: string, actorId: string): Promise<StatementIntent | null> {
    if (raw === null)
        return null;
    if (raw.length > 16384)
        throw new Error("Saved statement recovery is outside the supported bound. It was preserved.");
    let value: StatementIntent;
    try {
        value = JSON.parse(raw) as StatementIntent;
        if (!value || value.version !== 1 || value.tenantId !== tenantId || value.actorId !== actorId || typeof value.path !== "string" || typeof value.payload !== "string" || typeof value.key !== "string" || !statementUUID.test(value.key) || typeof value.integrity !== "string" || Object.keys(value).some(k => !["version", "tenantId", "actorId", "path", "kind", "key", "payload", "sourceSHA256", "integrity"].includes(k)))
            throw new Error();
        const route = statementRoute("POST", routeParts(value.path));
        if (!route || route.body !== value.kind || value.kind === "preview")
            throw new Error();
        const payload = JSON.parse(value.payload) as Record<string, unknown>;
        if (value.kind === "import") {
            if (Object.keys(payload).some(k => !["mapping_id", "source_sha256"].includes(k)) || typeof payload.mapping_id !== "string" || !statementUUID.test(payload.mapping_id) || payload.source_sha256 !== value.sourceSHA256 || !value.sourceSHA256 || !/^[0-9a-f]{64}$/.test(value.sourceSHA256))
                throw new Error();
        }
        else
            parseStatementBody(payload, value.kind);
        if (await integrity(value) !== value.integrity)
            throw new Error();
    }
    catch {
        throw new Error("Saved statement recovery is invalid or belongs to another identity. It was preserved.");
    }
    return value;
}
export async function statementRequestBody(intent: StatementIntent, source?: Uint8Array): Promise<string> {
    await parseStatementIntent(JSON.stringify(intent), intent.tenantId, intent.actorId);
    if (intent.kind !== "import")
        return intent.payload;
    if (!source || source.byteLength > maxStatementSourceBytes || await statementDigest(source) !== intent.sourceSHA256)
        throw new Error("Select the exact original file before retrying this import.");
    let binary = "";
    for (let n = 0; n < source.length; n += 32768)
        binary += String.fromCharCode(...source.subarray(n, n + 32768));
    const mapping = JSON.parse(intent.payload) as {
        mapping_id: string;
    };
    return JSON.stringify({ mapping_id: mapping.mapping_id, source_base64: btoa(binary) });
}
export function persistStatementIntent(storage: Storage, key: string, intent: StatementIntent): void {
    const serialized = JSON.stringify(intent);
    const previous = storage.getItem(key);
    if (previous !== null && previous !== serialized)
        throw new Error("Resolve the saved original statement command first.");
    storage.setItem(key, serialized);
    if (storage.getItem(key) !== serialized)
        throw new Error("Recovery storage did not confirm the original command. Nothing was sent.");
}
export function clearStatementIntent(storage: Storage, key: string, intent: StatementIntent): void {
    const previous = storage.getItem(key);
    if (previous !== JSON.stringify(intent)) {
        if (previous === null)
            return;
        throw new Error("A different saved command was preserved.");
    }
    storage.removeItem(key);
    if (storage.getItem(key) !== null)
        throw new Error("The confirmed command remains saved. Original-key replay is safe.");
}
