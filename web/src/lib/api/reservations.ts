// All financial values stay canonical integer strings in the browser.
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
export const reservationID = (v: unknown): v is string => typeof v === "string" && uuid.test(v) && v !== "00000000-0000-0000-0000-000000000000";
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v);
export const reservationMinor = (v: unknown): v is string => typeof v === "string" && /^(0|[1-9][0-9]{0,18})$/.test(v) && BigInt(v) <= 9223372036854775807n;
const date = (v: unknown): v is string => typeof v === "string" && v.length <= 40 && Number.isFinite(Date.parse(v));
export type ReservationCreate = Readonly<{
    debit_account_id: string;
    credit_account_id: string;
    amount_minor: string;
    currency: string;
    ttl_seconds: number;
}>;
export type ReservationDecision = Readonly<{
    decision: "capture" | "release";
}>;
export type ReservationReceipt = Readonly<{
    id: string;
    state: "active" | "captured" | "released";
    replayed: boolean;
    source_balance_version?: string;
    expires_at?: string;
    transfer_id?: string;
    rejection_code?: "expired_capture" | "capture_policy_denied";
}>;
export type Reservation = Readonly<{
    id: string;
    debit_account_id: string;
    credit_account_id: string;
    amount_minor: string;
    currency: string;
    state: "active" | "captured" | "released" | "expired";
    version: string;
    created_by: string;
    created_at: string;
    expires_at: string;
    completed_at?: string;
    transfer_id?: string;
}>;
export type ReservationPage = Readonly<{
    holds: readonly Reservation[];
    next_cursor?: string;
}>;
export function parseReservationBody(value: unknown, decision: boolean): ReservationCreate | ReservationDecision {
    if (!object(value))
        throw Error("Invalid reservation request.");
    if (decision) {
        if (Object.keys(value).length !== 1 || (value.decision !== "capture" && value.decision !== "release"))
            throw Error("Choose full capture or full release.");
        return { decision: value.decision };
    }
    const keys = ["debit_account_id", "credit_account_id", "amount_minor", "currency", "ttl_seconds"];
    if (Object.keys(value).length !== keys.length || Object.keys(value).some(k => !keys.includes(k)) || !reservationID(value.debit_account_id) || !reservationID(value.credit_account_id) || value.debit_account_id === value.credit_account_id || !reservationMinor(value.amount_minor) || value.amount_minor === "0" || typeof value.currency !== "string" || !/^[A-Z]{3}$/.test(value.currency) || typeof value.ttl_seconds !== "number" || !Number.isSafeInteger(value.ttl_seconds) || value.ttl_seconds < 1 || value.ttl_seconds > 86400)
        throw Error("Use two distinct accounts, exact positive minor units and an expiry from 1 to 86400 seconds.");
    return value as ReservationCreate;
}
export function isReservationReceipt(v: unknown): v is ReservationReceipt {
    if (!object(v) || !reservationID(v.id) || !["active", "captured", "released"].includes(String(v.state)) || typeof v.replayed !== "boolean")
        return false;
    if (v.rejection_code !== undefined)
        return v.state === "active" && ["expired_capture", "capture_policy_denied"].includes(String(v.rejection_code)) && v.transfer_id === undefined;
    if (!reservationMinor(v.source_balance_version))
        return false;
    if (v.state === "captured")
        return reservationID(v.transfer_id);
    if (v.transfer_id !== undefined)
        return false;
    return v.state === "released" || date(v.expires_at);
}
export function isReservation(v: unknown): v is Reservation {
    if (!object(v) || !reservationID(v.id) || !reservationID(v.debit_account_id) || !reservationID(v.credit_account_id) || v.debit_account_id === v.credit_account_id || !reservationMinor(v.amount_minor) || v.amount_minor === "0" || typeof v.currency !== "string" || !/^[A-Z]{3}$/.test(v.currency) || typeof v.created_by !== "string" || v.created_by.length > 256 || !date(v.created_at) || !date(v.expires_at))
        return false;
    if (v.state === "active")
        return v.version === "1" && v.completed_at === undefined && v.transfer_id === undefined;
    if (!["captured", "released", "expired"].includes(String(v.state)) || v.version !== "2" || !date(v.completed_at))
        return false;
    return v.state === "captured" ? reservationID(v.transfer_id) : v.transfer_id === undefined;
}
export function isReservationPage(v: unknown): v is ReservationPage { return object(v) && Array.isArray(v.holds) && v.holds.length <= 50 && v.holds.every(isReservation) && (v.next_cursor === undefined || typeof v.next_cursor === "string" && /^[A-Za-z0-9_-]{1,256}$/.test(v.next_cursor)); }
export function reservationRoute(method: string, parts: readonly string[]): {
    path: string;
    write: boolean;
    decision: boolean;
    result: "page" | "hold" | "receipt" | "csv";
} | null {
    if (parts.length === 0 && (method === "GET" || method === "POST"))
        return { path: "/api/reservations", write: method === "POST", decision: false, result: method === "GET" ? "page" : "receipt" };
    if (parts.length === 1 && parts[0] === "export.csv" && method === "GET")
        return { path: "/api/reservations/export.csv", write: false, decision: false, result: "csv" };
    if (parts.length === 1 && reservationID(parts[0]) && method === "GET")
        return { path: `/api/reservations/${parts[0]}`, write: false, decision: false, result: "hold" };
    if (parts.length === 2 && reservationID(parts[0]) && parts[1] === "decisions" && method === "POST")
        return { path: `/api/reservations/${parts[0]}/decisions`, write: true, decision: true, result: "receipt" };
    return null;
}
