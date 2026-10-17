"use client";
import { useEffect, useRef, useState } from "react";
import { isReservationReceipt, type ReservationReceipt } from "@/lib/api/reservations";
import { readBoundedBytes } from "@/lib/bounded-stream";
import { clearReservationIntent, createReservationIntent, parseReservationIntent, reservationStorageKey, saveReservationIntent, type ReservationIntent } from "./reservationIntent";
export function useReservationCommand(tenant: string, actor: string, csrf: string, online: boolean) {
    const key = reservationStorageKey(tenant, actor), flight = useRef(false);
    const [intent, setIntent] = useState<ReservationIntent | null>(null), [pending, setPending] = useState(false), [blocked, setBlocked] = useState(true), [rejected, setRejected] = useState(false), [error, setError] = useState<string | null>(null), [receipt, setReceipt] = useState<ReservationReceipt | null>(null);
    useEffect(() => {
        let active = true;
        void (async () => {
            try {
                const saved = await parseReservationIntent(sessionStorage.getItem(key), tenant, actor);
                if (active) {
                    setIntent(saved);
                    setBlocked(false);
                }
            }
            catch (cause) {
                if (active)
                    setError(cause instanceof Error ? cause.message : "Recovery storage is unavailable. Nothing was sent.");
            }
        })();
        return () => { active = false; };
    }, [actor, key, tenant]);
    async function dispatch(original: ReservationIntent) {
        saveReservationIntent(sessionStorage, key, original);
        setIntent(original);
        let response: Response, value: unknown;
        try {
            response = await fetch(original.path, { method: "POST", cache: "no-store", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf, "Idempotency-Key": original.key }, body: original.payload, signal: AbortSignal.timeout(15000) });
            value = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(await readBoundedBytes(response.body, 16384)));
        }
        catch {
            setError("The result is unknown. Keep and retry the original reservation request.");
            return null;
        }
        if (!response.ok) {
            const known = [400, 401, 403, 404, 409, 413, 415, 422, 429].includes(response.status);
            setRejected(known);
            setError(response.status === 409 ? "The original intent or current hold state conflicts. Read current evidence before another decision." : known ? "The server rejected this request. Review current evidence. The original request stays available." : "The result is unknown. Retry the original request.");
            return null;
        }
        if (!isReservationReceipt(value)) {
            setError("No valid receipt arrived. Keep and retry the original reservation request.");
            return null;
        }
        setReceipt(value);
        setRejected(false);
        try {
            clearReservationIntent(sessionStorage, key, original);
            setIntent(null);
        }
        catch (cause) {
            setError(cause instanceof Error ? cause.message : "The command is confirmed. Recovery cleanup needs attention.");
        }
        return value;
    }
    async function execute(path?: string, body?: unknown) {
        if (flight.current || pending || blocked || !online || path && intent)
            return null;
        flight.current = true;
        setPending(true);
        setError(null);
        setReceipt(null);
        setRejected(false);
        try {
            const saved = await parseReservationIntent(sessionStorage.getItem(key), tenant, actor);
            if (saved && path) {
                setIntent(saved);
                setError("Resolve the saved original request first.");
                return null;
            }
            const original = saved ?? (path ? await createReservationIntent(tenant, actor, path, body) : null);
            if (!original) {
                setIntent(null);
                setError("No original request is saved. Read current evidence.");
                return null;
            }
            return await dispatch(original);
        }
        catch (cause) {
            setBlocked(true);
            setError(cause instanceof Error ? cause.message : "Recovery storage is unavailable. Nothing was sent.");
            return null;
        }
        finally {
            flight.current = false;
            setPending(false);
        }
    }
    function closeRejected() {
        if (!rejected || !intent || pending)
            return;
        try {
            clearReservationIntent(sessionStorage, key, intent);
            setIntent(null);
            setRejected(false);
            setError(null);
        }
        catch (cause) {
            setError(cause instanceof Error ? cause.message : "The original request was preserved.");
        }
    }
    return { intent, pending, blocked, rejected, error, receipt, execute, closeRejected };
}
