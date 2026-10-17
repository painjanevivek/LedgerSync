import { parseReservationBody, reservationID, reservationRoute } from "@/lib/api/reservations";
export type ReservationIntent = Readonly<{
    version: 1;
    tenant: string;
    actor: string;
    path: string;
    key: string;
    payload: string;
    integrity: string;
}>;
export const reservationStorageKey = (tenant: string, actor: string) => `ledgersync:reservation-command:v1:${encodeURIComponent(tenant)}:${encodeURIComponent(actor)}`;
async function digest(intent: Omit<ReservationIntent, "integrity">) { const bytes = new TextEncoder().encode(JSON.stringify([intent.version, intent.tenant, intent.actor, intent.path, intent.key, intent.payload])); return Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", bytes)), n => n.toString(16).padStart(2, "0")).join(""); }
function route(path: string) {
    if (!path.startsWith("/api/reservations"))
        throw Error("Invalid reservation destination.");
    const suffix = path.slice("/api/reservations".length);
    if (suffix !== "" && !suffix.startsWith("/"))
        throw Error("Invalid reservation destination.");
    const result = reservationRoute("POST", suffix ? suffix.slice(1).split("/") : []);
    if (!result || result.path !== path)
        throw Error("Invalid reservation destination.");
    return result;
}
export async function createReservationIntent(tenant: string, actor: string, path: string, body: unknown): Promise<ReservationIntent> { const target = route(path), payload = JSON.stringify(parseReservationBody(body, target.decision)), value = { version: 1 as const, tenant, actor, path, key: crypto.randomUUID(), payload }; return { ...value, integrity: await digest(value) }; }
export async function parseReservationIntent(raw: string | null, tenant: string, actor: string): Promise<ReservationIntent | null> {
    if (raw === null)
        return null;
    try {
        if (raw.length > 8192)
            throw Error();
        const value = JSON.parse(raw) as ReservationIntent;
        if (!value || value.version !== 1 || value.tenant !== tenant || value.actor !== actor || typeof value.path !== "string" || !reservationID(value.key) || typeof value.payload !== "string" || typeof value.integrity !== "string" || Object.keys(value).some(k => !["version", "tenant", "actor", "path", "key", "payload", "integrity"].includes(k)))
            throw Error();
        const target = route(value.path);
        parseReservationBody(JSON.parse(value.payload), target.decision);
        if (await digest(value) !== value.integrity)
            throw Error();
        return value;
    }
    catch {
        throw Error("Saved reservation recovery is invalid or belongs to another identity. It was preserved. New commands are blocked.");
    }
}
export function saveReservationIntent(storage: Storage, key: string, value: ReservationIntent) {
    const raw = JSON.stringify(value), previous = storage.getItem(key);
    if (previous !== null && previous !== raw)
        throw Error("Resolve the original reservation request first.");
    storage.setItem(key, raw);
    if (storage.getItem(key) !== raw)
        throw Error("Recovery storage did not confirm the request. Nothing was sent.");
}
export function clearReservationIntent(storage: Storage, key: string, value: ReservationIntent) {
    const previous = storage.getItem(key);
    if (previous === null)
        return;
    if (previous !== JSON.stringify(value))
        throw Error("A different saved reservation was preserved.");
    storage.removeItem(key);
    if (storage.getItem(key) !== null)
        throw Error("The confirmed original request remains saved. Original-key replay is safe.");
}
