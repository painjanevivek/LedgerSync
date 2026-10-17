"use client";
import { useCallback, useRef, useState } from "react";
import { isStatementRunPage, isStatementWorkspace, type StatementRunPage, type StatementWorkspace } from "@/lib/api/statement-reconciliation";
import { readStatementBytes } from "@/lib/statement-stream";
async function readEvidence(path: string): Promise<unknown> {
    const response = await fetch(path, { cache: "no-store", signal: AbortSignal.timeout(10000) });
    if (!response.ok)
        throw new Error(response.status === 403 ? "Your session cannot read this statement evidence." : response.status === 404 ? "This statement record is not available in your tenant." : "Statement evidence is unavailable. No empty or matched result is inferred.");
    return JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(await readStatementBytes(response.body, 1048576))) as unknown;
}
export function useStatementWorkspace() {
    const [workspace, setWorkspace] = useState<StatementWorkspace | null>(null), [page, setPage] = useState<StatementRunPage | null>(null);
    const [workspaceLoading, setWorkspaceLoading] = useState(false), [runLoading, setRunLoading] = useState(false), [workspaceError, setWorkspaceError] = useState<string | null>(null), [runError, setRunError] = useState<string | null>(null), [cursor, setCursor] = useState("");
    const workspaceGeneration = useRef(0), runGeneration = useRef(0);
    const refresh = useCallback(async () => {
        const generation = ++workspaceGeneration.current;
        setWorkspaceLoading(true);
        setWorkspaceError(null);
        try {
            const value = await readEvidence("/api/statement-reconciliation");
            if (!isStatementWorkspace(value))
                throw new Error("The statement workspace response is invalid.");
            if (generation === workspaceGeneration.current)
                setWorkspace(value);
            return value;
        }
        catch (cause) {
            if (generation === workspaceGeneration.current)
                setWorkspaceError(cause instanceof Error ? cause.message : "Statement evidence is unavailable.");
            return null;
        }
        finally {
            if (generation === workspaceGeneration.current)
                setWorkspaceLoading(false);
        }
    }, []);
    const loadRun = useCallback(async (id: string, next = "") => {
        const generation = ++runGeneration.current;
        setRunLoading(true);
        setRunError(null);
        setCursor(next);
        setPage(previous => previous?.run.id === id ? previous : null);
        try {
            const value = await readEvidence(`/api/statement-reconciliation/runs/${id}${next ? "?cursor=" + encodeURIComponent(next) : ""}`);
            if (!isStatementRunPage(value) || value.run.id !== id)
                throw new Error("The statement run response is invalid.");
            if (generation === runGeneration.current)
                setPage(value);
        }
        catch (cause) {
            if (generation === runGeneration.current)
                setRunError(cause instanceof Error ? cause.message : "Statement run evidence is unavailable.");
        }
        finally {
            if (generation === runGeneration.current)
                setRunLoading(false);
        }
    }, []);
    return { workspace, page, cursor, workspaceLoading, runLoading, workspaceError, runError, refresh, loadRun };
}
