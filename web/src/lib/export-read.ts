import { NextRequest, NextResponse } from "next/server";

import { createRateLimitStore, rateLimitResponse } from "@/lib/rate-limit";
import { hasValidHost, jsonError } from "@/lib/security";
import type { Session } from "@/lib/session";
import { isPrivateAPITimeout } from "@/lib/upstream-outcome";
import { parseTransferSearchParams, transferExportQueryRules } from "@/lib/page-query/transfers";

const exportRateLimit = createRateLimitStore();
const maximumCSVBytes = 16 * 1024 * 1024;
const maximumErrorBytes = 65_536;
const exportTimeoutMilliseconds = 12_000;
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const publicErrors = new Set(["validation_failed", "unauthorized", "forbidden", "not_found", "rate_limited", "export_unavailable", "export_timeout"]);

type ExportFamily = "transfers" | "account-ledger" | "reconciliation";

export async function authorizeEvidenceExport(request: NextRequest, session: Session | null, additionalScope: "transfers:read" | "transactions:read" | "reconciliation:read") {
  if (request.method !== "GET") { const response = jsonError("method_not_allowed", 405); response.headers.set("Allow", "GET"); return response; }
  if (!session) return jsonError("unauthorized", 401);
  if (!session.scopes?.includes("exports:read") || !session.scopes.includes(additionalScope)) return jsonError("forbidden", 403);
  if (!hasValidHost(request)) return jsonError("invalid_request", 400);
  const decision = await exportRateLimit.consume(`export:${session.tenantId}:${session.subjectId}`, 10, 60);
  const rateLimitFailure = rateLimitResponse(decision);
  if (rateLimitFailure) return rateLimitFailure;
  return session;
}

export function isEvidenceExportDenial(value: Session | NextResponse): value is NextResponse { return value instanceof NextResponse; }

function strictDate(value: string) { return /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.test(value) && !Number.isNaN(Date.parse(value)); }

export function strictExportQuery(request: NextRequest, family: "transfers" | "account" | "reconciliation") {
  if (family === "transfers") {
    const parsed = parseTransferSearchParams(request.nextUrl.searchParams, transferExportQueryRules);
    if (!parsed.ok) return jsonError("validation_failed", 400);
    const output = new URLSearchParams();
    for (const key of ["accountId", "status", "q", "from", "to", "limit"] as const) {
      const value = parsed.values[key];
      if (value) output.set(key, value);
    }
    return output;
  }
  const allowed = family === "reconciliation" ? ["runId", "status", "from", "to", "limit"] : ["limit"];
  const output = new URLSearchParams();
  for (const [key] of request.nextUrl.searchParams) if (!allowed.includes(key) || request.nextUrl.searchParams.getAll(key).length !== 1) return jsonError("validation_failed", 400);
  for (const key of allowed) {
    const raw = request.nextUrl.searchParams.get(key);
    if (raw === null || raw === "") continue;
    const value = raw.trim();
    if (!value || /[\u0000-\u001f\u007f-\u009f]/u.test(value)) return jsonError("validation_failed", 400);
    if (key === "limit" && (!/^[1-9][0-9]{0,4}$/.test(value) || Number(value) > 10_000)) return jsonError("validation_failed", 400);
    if ((key === "accountId" || key === "runId") && !uuid.test(value)) return jsonError("validation_failed", 400);
    if ((key === "from" || key === "to") && !strictDate(value)) return jsonError("validation_failed", 400);
    if (key === "q" && value.length > 128) return jsonError("validation_failed", 400);
    if (key === "status" && !["running", "matched", "mismatch", "failed"].includes(value)) return jsonError("validation_failed", 400);
    output.set(key, value);
  }
  const from = output.get("from"), to = output.get("to");
  if (from && to && Date.parse(from) > Date.parse(to)) return jsonError("validation_failed", 400);
  return output;
}

async function readBoundedError(response: Response) {
  const advertised = response.headers.get("content-length");
  if (advertised !== null && (!/^\d+$/.test(advertised) || Number(advertised) > maximumErrorBytes)) throw new Error("response_too_large");
  if (!response.body) return "";
  const reader = response.body.getReader(); const decoder = new TextDecoder("utf-8", { fatal: true }); let received = 0, text = "";
  try { while (true) { const chunk = await reader.read(); if (chunk.done) break; received += chunk.value.byteLength; if (received > maximumErrorBytes) throw new Error("response_too_large"); text += decoder.decode(chunk.value, { stream: true }); } return text + decoder.decode(); }
  finally { reader.releaseLock(); }
}

function safeError(status: number, raw: string, upstreamHeaders?: Headers) {
  let code = "export_unavailable";
  try { const value = JSON.parse(raw) as { error?: { code?: unknown } }; if (typeof value.error?.code === "string" && publicErrors.has(value.error.code)) code = value.error.code; } catch { /* The private body is never exposed. */ }
  const safeStatus = [400, 401, 403, 404, 429, 503, 504].includes(status) ? status : 503;
  const response = jsonError(code, safeStatus);
  const retryAfter = upstreamHeaders?.get("retry-after"); if (retryAfter && /^\d{1,6}$/.test(retryAfter)) response.headers.set("Retry-After", retryAfter);
  return response;
}

export function sanitizeExportHeaders(headers: Headers, family: ExportFamily) {
  const type = headers.get("content-type")?.toLowerCase();
  const disposition = headers.get("content-disposition");
  const schema = headers.get("x-ledgersync-export-schema");
  const pattern = new RegExp(`^attachment; filename="ledgersync-${family}-\\d{8}T\\d{6}Z-v2\\.csv"$`);
  if (!type || !/^text\/csv(?:;\s*charset=utf-8)?$/.test(type) || !disposition || !pattern.test(disposition) || schema !== "2") return null;
  const result: Record<string, string> = { "Cache-Control": "no-store", "Content-Type": "text/csv; charset=utf-8", "Content-Disposition": disposition, "X-LedgerSync-Export-Schema": "2", "X-Content-Type-Options": "nosniff" };
  const requestID = headers.get("x-request-id"); if (requestID && /^[A-Za-z0-9._:-]{1,128}$/.test(requestID)) result["X-Request-ID"] = requestID;
  const length = headers.get("content-length"); if (length && /^\d+$/.test(length) && Number(length) <= maximumCSVBytes) result["Content-Length"] = length;
  return result;
}

export async function proxyEvidenceExport(session: Session, path: string, query: URLSearchParams, family: ExportFamily): Promise<Response> {
  let connection: Readonly<{apiURL:string;headers:Record<string,string>}>;
  try { const {privateAPIContext}=await import("@/lib/private-api"); connection = await privateAPIContext(session); } catch { return jsonError("export_unavailable", 503); }
  const controller = new AbortController(); const timeout = setTimeout(() => controller.abort(new DOMException("Export timed out", "TimeoutError")), exportTimeoutMilliseconds);
  let upstream: Response;
  try { upstream = await fetch(`${connection.apiURL}${path}${query.size ? `?${query}` : ""}`, { headers: { ...connection.headers, Accept: "text/csv" }, cache: "no-store", signal: controller.signal }); }
  catch (error) { clearTimeout(timeout); return jsonError(isPrivateAPITimeout(error) ? "export_timeout" : "export_unavailable", isPrivateAPITimeout(error) ? 504 : 503); }
  if (!upstream.ok) { try { const raw = await readBoundedError(upstream); clearTimeout(timeout); return safeError(upstream.status, raw, upstream.headers); } catch { clearTimeout(timeout); return jsonError("export_unavailable", 503); } }
  const advertised = upstream.headers.get("content-length");
  if (advertised !== null && (!/^\d+$/.test(advertised) || Number(advertised) > maximumCSVBytes)) { clearTimeout(timeout); await upstream.body?.cancel(); return jsonError("export_unavailable", 503); }
  const headers = sanitizeExportHeaders(upstream.headers, family);
  if (!headers || !upstream.body) { clearTimeout(timeout); await upstream.body?.cancel(); return jsonError("export_unavailable", 503); }
  const reader = upstream.body.getReader(); let received = 0;
  const stream = new ReadableStream<Uint8Array>({
    async pull(target) { try { const chunk = await reader.read(); if (chunk.done) { clearTimeout(timeout); target.close(); return; } received += chunk.value.byteLength; if (received > maximumCSVBytes) throw new Error("response_too_large"); target.enqueue(chunk.value); } catch (error) { clearTimeout(timeout); controller.abort(); target.error(error); } },
    async cancel(reason) { clearTimeout(timeout); controller.abort(); await reader.cancel(reason); },
  });
  return new Response(stream, { status: 200, headers });
}
