"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { isReservation, isReservationPage, type Reservation, type ReservationPage } from "@/lib/api/reservations";
import { readBoundedBytes } from "@/lib/bounded-stream";
export function useReservationEvidence(online: boolean) {
    const [page, setPage] = useState<ReservationPage | null>(null), [current, setCurrent] = useState<Reservation | null>(null), [error, setError] = useState<string | null>(null), [busy, setBusy] = useState(false), [cursor, setCursor] = useState("");
    const generation = useRef(0);
    const read = useCallback(async (next = "", id?: string) => {
        const own = ++generation.current;
        setBusy(true);
        setError(null);
        try {
            const fetchJSON = async (path: string) => {
                const response = await fetch(path, { cache: "no-store", signal: AbortSignal.timeout(10000) });
                if (!response.ok)
                    throw Error("The authorized reservation read is unavailable. Earlier evidence is historical.");
                return JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(await readBoundedBytes(response.body, 131072))) as unknown;
            };
            const [listed, selected] = await Promise.all([fetchJSON("/api/reservations" + (next ? "?cursor=" + encodeURIComponent(next) : "")), id ? fetchJSON("/api/reservations/" + id) : Promise.resolve(null)]);
            if (!isReservationPage(listed) || selected !== null && !isReservation(selected))
                throw Error("The reservation response is invalid. No empty or completed result was inferred.");
            if (own === generation.current) {
                setPage(listed);
                setCursor(next);
                setCurrent(selected);
            }
        }
        catch (cause) {
            if (own === generation.current)
                setError(cause instanceof Error ? cause.message : "Reservation evidence is unavailable.");
        }
        finally {
            if (own === generation.current)
                setBusy(false);
        }
    }, []);
    const invalidate=useCallback(()=>{generation.current++;},[]);
    useEffect(() => {
        if (!online)
            return;
        const timer = window.setTimeout(() => void read(), 0);
        return () => { window.clearTimeout(timer); invalidate(); };
    }, [online, read, invalidate]);
    return { page, current, error, busy, cursor, read };
}
