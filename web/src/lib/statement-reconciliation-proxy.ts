import { NextRequest, NextResponse } from "next/server";
import { isStatementPreview, isStatementReceipt, isStatementRunPage, isStatementWorkspace, maxStatementRequestBytes, maxStatementSourceBytes, parseStatementBody, statementRoute, } from "@/lib/api/statement-reconciliation";
import { privateAPIContext } from "@/lib/private-api";
import { createRateLimitStore, type RateLimitStore } from "@/lib/rate-limit";
import { authorizeReconciliationMutation, isReconciliationMutationDenial, reconciliationPrivateHeaders, reconciliationResponseHeaders } from "@/lib/reconciliation-mutation-boundary";
import { jsonError } from "@/lib/security";
import type { Session } from "@/lib/session";
import { privateWriteTimeoutMilliseconds } from "@/lib/upstream-outcome";
const baseRateLimit = createRateLimitStore();
const rateLimit: RateLimitStore = { consume: (key, limit, windowSeconds) => baseRateLimit.consume("statement:" + key, limit, windowSeconds) };
const knownErrors = new Set(["statement_validation_failed", "statement_conflict", "statement_limit", "statement_unavailable", "invalid_request", "unauthorized", "forbidden", "not_found", "rate_limited", "temporary_unavailable", "unsupported_media_type"]);
import { readStatementBytes } from "@/lib/statement-stream";
function querySuffix(request: NextRequest, allowed: boolean): string {
    const params = request.nextUrl.searchParams;
    for (const [key, value] of params)
        if (!allowed || key !== "cursor" || params.getAll(key).length !== 1 || !/^[A-Za-z0-9_-]{1,512}$/.test(value))
            throw new Error("invalid statement query");
    return params.size ? "?" + params.toString() : "";
}
function jsonValue(bytes: Uint8Array): unknown { return JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes)) as unknown; }
function errorResponse(status: number, value: unknown, headers: Record<string, string>): NextResponse {
    const error = typeof value === "object" && value !== null ? (value as {
        error?: {
            code?: unknown;
            message?: unknown;
        };
    }).error : undefined;
    const code = typeof error?.code === "string" && knownErrors.has(error.code) ? error.code : "statement_unavailable";
    const safeStatus = [400, 401, 403, 404, 409, 413, 415, 422, 429, 503].includes(status) ? status : 503;
    const message = typeof error?.message === "string" && /^statement row [0-9]{1,4}: [a-z_]+$/.test(error.message) ? error.message : undefined;
    return NextResponse.json({ error: { code, ...(message ? { message } : {}) } }, { status: safeStatus, headers });
}
export async function proxyStatement(request: NextRequest, session: Session | null, parts: readonly string[]): Promise<NextResponse> {
    const route = statementRoute(request.method, parts);
    if (!route)
        return jsonError("not_found", 404);
    if (!session)
        return jsonError("unauthorized", 401);
    if (!session.scopes?.includes(route.write ? "reconciliation:write" : "reconciliation:read") || route.export && !session.scopes?.includes("exports:read"))
        return jsonError("forbidden", 403);
    let suffix: string;
    try {
        suffix = querySuffix(request, route.cursor === true);
    }
    catch {
        return jsonError("invalid_request", 400);
    }
    let body: string | undefined, key: string | undefined;
    if (route.write) {
        const authorized = await authorizeReconciliationMutation(request, session, rateLimit);
        if (isReconciliationMutationDenial(authorized))
            return authorized;
        key = authorized.idempotencyKey;
        try {
            const length = request.headers.get("content-length");
            if (length !== null && (!/^[1-9][0-9]*$/.test(length) || Number(length) > maxStatementRequestBytes))
                return jsonError("statement_validation_failed", 400);
            const raw = await readStatementBytes(request.body, maxStatementRequestBytes);
            body = JSON.stringify(parseStatementBody(jsonValue(raw), route.body!));
        }
        catch {
            return jsonError("statement_validation_failed", 400);
        }
    }
    let connection: Awaited<ReturnType<typeof privateAPIContext>>;
    try {
        connection = await privateAPIContext(session);
    }
    catch {
        return jsonError("temporary_unavailable", 503);
    }
    let upstream: Response;
    try {
        upstream = await fetch(connection.apiURL + route.path + suffix, { method: request.method, headers: route.write ? reconciliationPrivateHeaders(connection.headers, key!) : connection.headers, body, cache: "no-store", signal: AbortSignal.timeout(privateWriteTimeoutMilliseconds) });
    }
    catch {
        return jsonError(route.write ? "statement_outcome_unknown" : "statement_unavailable", route.write ? 504 : 503);
    }
    const headers = reconciliationResponseHeaders(upstream.headers);
    let bytes: Uint8Array;
    try {
        bytes = await readStatementBytes(upstream.body, upstream.ok ? route.source ? maxStatementSourceBytes : 4 * maxStatementSourceBytes : 16384);
    }
    catch {
        return jsonError(route.write ? "statement_outcome_unknown" : "statement_unavailable", route.write ? 504 : 503);
    }
    if (!upstream.ok) {
        try {
            return errorResponse(upstream.status, jsonValue(bytes), headers);
        }
        catch {
            return jsonError("statement_unavailable", 503);
        }
    }
    if (route.result === "csv") {
        const audit = upstream.headers.get("x-audit-reference"), digest = upstream.headers.get("x-statement-sha256");
        if (!audit || !/^[0-9a-f-]{36}$/.test(audit) || route.source && (!digest || !/^[0-9a-f]{64}$/.test(digest)))
            return jsonError("statement_unavailable", 503);
        return new NextResponse(bytes as BodyInit, { status: 200, headers: { ...headers, "Content-Type": "text/csv; charset=utf-8", "Content-Disposition": `attachment; filename="statement-${route.resourceID}.csv"`, "X-Content-Type-Options": "nosniff", "X-Audit-Reference": audit, ...(digest ? { "X-Statement-SHA256": digest } : {}) } });
    }
    try {
        const value = jsonValue(bytes);
        const valid = route.result === "workspace" ? isStatementWorkspace(value) : route.result === "preview" ? isStatementPreview(value) : route.result === "page" ? isStatementRunPage(value) : isStatementReceipt(value);
        if (!valid)
            throw new Error("invalid statement evidence");
        return NextResponse.json(value, { status: 200, headers });
    }
    catch {
        return jsonError(route.write ? "statement_outcome_unknown" : "statement_unavailable", route.write ? 504 : 503);
    }
}
