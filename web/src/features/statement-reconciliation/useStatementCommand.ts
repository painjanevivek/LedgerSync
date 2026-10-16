"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { isStatementReceipt, type StatementBodyKind, type StatementReceipt } from "@/lib/api/statement-reconciliation";
import { clearStatementIntent, createStatementIntent, parseStatementIntent, persistStatementIntent, statementRequestBody, statementStorageKey, type StatementIntent } from "./statementIntent";
export type StatementCommandResult = Readonly<{
    receipt: StatementReceipt;
    path: string;
    kind: StatementBodyKind;
}>;
export function useStatementCommand(tenantId: string, actorId: string, csrfToken: string, online: boolean) {
    const storageKey = statementStorageKey(tenantId, actorId), inFlight = useRef(false);
    const [intent, setIntent] = useState<StatementIntent | null>(null), [pending, setPending] = useState(false), [error, setError] = useState<string | null>(null), [blocked, setBlocked] = useState(false), [rejected, setRejected] = useState(false), [result, setResult] = useState<StatementCommandResult | null>(null);
    useEffect(() => {
        let active = true;
        const load = async () => {
            try {
                const saved = await parseStatementIntent(sessionStorage.getItem(storageKey), tenantId, actorId);
                if (active) {
                    setIntent(saved);
                    setBlocked(false);
                    setRejected(false);
                }
            }
            catch (cause) {
                if (active) {
                    setBlocked(true);
                    setError(cause instanceof Error ? cause.message : "Recovery storage is unavailable. Nothing was sent.");
                }
            }
        };
        void load();
        return () => { active = false; };
    }, [actorId, storageKey, tenantId]);
    const dispatch = useCallback(async (original: StatementIntent, source?: Uint8Array): Promise<StatementCommandResult | null> => {
        let body: string;
        try {
            body = await statementRequestBody(original, source);
            persistStatementIntent(sessionStorage, storageKey, original);
        }
        catch (cause) {
            setError(cause instanceof Error ? cause.message : "Recovery storage is unavailable. Nothing was sent.");
            return null;
        }
        let response: Response;
        try {
            response = await fetch(original.path, { method: "POST", cache: "no-store", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "Idempotency-Key": original.key }, body, signal: AbortSignal.timeout(15000) });
        }
        catch {
            setError("The result is unknown. Keep this original request and retry it when the connection returns.");
            return null;
        }
        let value: unknown;
        try {
            value = await response.json();
        }
        catch {
            setError("The result is unknown because no valid receipt arrived. Keep the original request.");
            return null;
        }
        if (!response.ok) {
            const knownRejection = [400, 401, 403, 404, 409, 413, 415, 422, 429].includes(response.status);
            setRejected(knownRejection);
            const code = (value as {
                error?: {
                    code?: unknown;
                };
            })?.error?.code;
            setError(response.status === 409 ? "The saved intent or review version conflicts. Read current evidence before another decision." : response.status === 429 ? "The request limit was reached. Wait, then retry this original request." : knownRejection ? `The server rejected this request (${typeof code === "string" ? code : "invalid_request"}). The original request stays available for review.` : "The result is unknown. Retry the original request; do not create a replacement.");
            return null;
        }
        if (!isStatementReceipt(value)) {
            setError("The result is unknown because the receipt is invalid. Keep the original request.");
            return null;
        }
        const confirmed = { receipt: value, path: original.path, kind: original.kind };
        setResult(confirmed);
        setRejected(false);
        try {
            clearStatementIntent(sessionStorage, storageKey, original);
            setIntent(null);
        }
        catch (cause) {
            setError(cause instanceof Error ? cause.message : "The command succeeded, but recovery cleanup needs attention.");
        }
        return confirmed;
    }, [csrfToken, storageKey]);
    const run = useCallback(async (kind: StatementBodyKind, path: string, body: unknown, source?: Uint8Array, sourceSHA256?: string) => {
        if (inFlight.current || pending || blocked || intent || !online)
            return null;
        inFlight.current = true;
        setPending(true);
        setError(null);
        setResult(null);
        setRejected(false);
        try {
            const saved = await parseStatementIntent(sessionStorage.getItem(storageKey), tenantId, actorId);
            if (saved) {
                setIntent(saved);
                setError("Resolve the saved original statement command first.");
                return null;
            }
            const original = await createStatementIntent(tenantId, actorId, path, kind, body, sourceSHA256);
            persistStatementIntent(sessionStorage, storageKey, original);
            setIntent(original);
            return await dispatch(original, source);
        }
        catch (cause) {
            setBlocked(true);
            setError(cause instanceof Error ? cause.message : "Recovery storage did not confirm the original request. Nothing was sent.");
            return null;
        }
        finally {
            inFlight.current = false;
            setPending(false);
        }
    }, [actorId, blocked, dispatch, intent, online, pending, storageKey, tenantId]);
    const retry = useCallback(async (source?: Uint8Array) => {
        if (inFlight.current || pending || blocked || !online)
            return null;
        inFlight.current = true;
        setPending(true);
        setError(null);
        setRejected(false);
        try {
            const original = await parseStatementIntent(sessionStorage.getItem(storageKey), tenantId, actorId);
            if (!original) {
                setIntent(null);
                setError("No saved original request is available. Read current evidence before another command.");
                return null;
            }
            setIntent(original);
            return await dispatch(original, source);
        }
        catch (cause) {
            setBlocked(true);
            setError(cause instanceof Error ? cause.message : "The saved original request is unavailable.");
            return null;
        }
        finally {
            inFlight.current = false;
            setPending(false);
        }
    }, [actorId, blocked, dispatch, online, pending, storageKey, tenantId]);
    const closeRejected = useCallback(() => {
        if (!rejected || !intent || pending)
            return;
        try {
            clearStatementIntent(sessionStorage, storageKey, intent);
            setIntent(null);
            setRejected(false);
            setError(null);
        }
        catch (cause) {
            setError(cause instanceof Error ? cause.message : "Recovery was preserved.");
        }
    }, [intent, pending, rejected, storageKey]);
    return { intent, pending, error, blocked, rejected, result, run, retry, closeRejected };
}
