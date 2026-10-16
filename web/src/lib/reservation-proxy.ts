import { NextRequest, NextResponse } from "next/server";
import { isReservation, isReservationPage, isReservationReceipt, parseReservationBody, reservationRoute } from "@/lib/api/reservations";
import { isValidReconciliationIdempotencyKey } from "@/lib/api/reconciliation";
import { privateAPIContext } from "@/lib/private-api";
import { createRateLimitStore, rateLimitResponse } from "@/lib/rate-limit";
import { reconciliationPrivateHeaders, reconciliationResponseHeaders } from "@/lib/reconciliation-mutation-boundary";
import { hasValidCSRF, jsonError } from "@/lib/security";
import type { Session } from "@/lib/session";
import { readBoundedBytes } from "@/lib/bounded-stream";
import { privateWriteTimeoutMilliseconds } from "@/lib/upstream-outcome";
const rate = createRateLimitStore();
const codes = new Set(["invalid_request", "forbidden", "unauthorized", "not_found", "rate_limited", "reservation_conflict", "reservation_limit", "reservation_export_limit", "reservation_unavailable", "temporary_unavailable"]);
const json = (raw: Uint8Array): unknown => JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(raw));
export async function proxyReservation(request: NextRequest, session: Session | null, parts: readonly string[]): Promise<NextResponse> {
    const route = reservationRoute(request.method, parts);
    if (!route)
        return jsonError("not_found", 404);
    if (!session)
        return jsonError("unauthorized", 401);
    if (!session.scopes?.includes(route.write ? "transfers:write" : "transfers:read"))
        return jsonError("forbidden", 403);
    if (route.result === "csv" && !session.scopes?.includes("exports:read"))
        return jsonError("forbidden", 403);
    for (const [key, value] of request.nextUrl.searchParams)
        if (route.result !== "page" || key !== "cursor" || request.nextUrl.searchParams.getAll(key).length !== 1 || !/^[A-Za-z0-9_-]{1,256}$/.test(value))
            return jsonError("invalid_request", 400);
    let body: string | undefined, key: string | undefined;
    if (route.write) {
        if (!hasValidCSRF(request, session))
            return jsonError("csrf_failed", 403);
        key = request.headers.get("idempotency-key") ?? undefined;
        if (!isValidReconciliationIdempotencyKey(key ?? null))
            return jsonError("idempotency_key_required", 400);
        if (request.headers.get("content-type")?.split(";", 1)[0].trim().toLowerCase() !== "application/json")
            return jsonError("unsupported_media_type", 415);
        const denied = rateLimitResponse(await rate.consume(`reservations:${session.tenantId}:${session.subjectId}`, 6, 60));
        if (denied)
            return denied;
        try {
            body = JSON.stringify(parseReservationBody(json(await readBoundedBytes(request.body, 4096)), route.decision));
        }
        catch {
            return jsonError("invalid_request", 400);
        }
    }
    let connection: Awaited<ReturnType<typeof privateAPIContext>>;
    try {
        connection = await privateAPIContext(session);
    }
    catch {
        return jsonError("temporary_unavailable", 503);
    }
    try {
        const upstream = await fetch(connection.apiURL + route.path + request.nextUrl.search, { method: request.method, cache: "no-store", body, headers: route.write ? reconciliationPrivateHeaders(connection.headers, key!) : connection.headers, signal: AbortSignal.timeout(privateWriteTimeoutMilliseconds) });
        const headers = reconciliationResponseHeaders(upstream.headers), bytes = await readBoundedBytes(upstream.body, upstream.ok ? route.result === "csv" ? 2097152 : 131072 : 16384);
        if (upstream.ok && route.result === "csv") {
            const audit = upstream.headers.get("x-audit-reference");
            if (!upstream.headers.get("content-type")?.startsWith("text/csv") || !audit || !/^[0-9a-f-]{36}$/.test(audit)) throw Error("Invalid audited CSV evidence");
            return new NextResponse(bytes as BodyInit, {status: 200, headers: {...headers, "Content-Type": "text/csv; charset=utf-8", "Content-Disposition": 'attachment; filename="reservation-obligations.csv"', "X-Content-Type-Options": "nosniff", "X-Audit-Reference": audit}});
        }
        const value = json(bytes);
        if (!upstream.ok) {
            const code = (value as {
                error?: {
                    code?: unknown;
                };
            })?.error?.code;
            return NextResponse.json({ error: { code: typeof code === "string" && codes.has(code) ? code : "reservation_unavailable" } }, { status: [400, 401, 403, 404, 409, 413, 415, 422, 429, 503].includes(upstream.status) ? upstream.status : 503, headers });
        }
        if (!(route.result === "page" ? isReservationPage(value) : route.result === "hold" ? isReservation(value) : isReservationReceipt(value)))
            throw Error("Invalid reservation evidence");
        return NextResponse.json(value, { status: 200, headers });
    }
    catch {
        return jsonError(route.write ? "reservation_outcome_unknown" : "reservation_unavailable", route.write ? 504 : 503);
    }
}
