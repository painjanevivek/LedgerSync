import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { parseWorkspaceStatusInput } from "@/lib/api/investigation-workspaces";
import { canonicalUUID } from "@/lib/canonical-uuid";
import { authorizeInvestigationEvidenceBundle, isInvestigationSearchDenial } from "@/lib/investigation-search-boundary";
import { privateAPIContext } from "@/lib/private-api";
import { InMemoryRateLimitStore } from "@/lib/rate-limit";
import { jsonError, readBoundedJSON } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";
import { isPrivateAPITimeout, privateWriteTimeoutMilliseconds } from "@/lib/upstream-outcome";

const rateLimit = new InMemoryRateLimitStore();
const maximumRequestBytes = 4 << 10;
const maximumBundleBytes = 512 * 1024;
const maximumErrorBytes = 65_536;
const digest = /^[0-9a-f]{64}$/u;
const timestamp = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/u;
const publicErrors = new Set(["invalid_request", "unauthorized", "forbidden", "not_found", "rate_limited", "export_unavailable", "investigation_version_conflict"]);

async function boundedBytes(response: Response, maximum: number) {
  const declared = response.headers.get("content-length");
  if (declared !== null && (!/^\d+$/u.test(declared) || Number(declared) > maximum)) throw new Error("response_too_large");
  const content = new Uint8Array(await response.arrayBuffer());
  if (content.byteLength > maximum) throw new Error("response_too_large");
  return content;
}

function safeError(status: number, content: Uint8Array, headers: Headers) {
  let code = "export_unavailable";
  try { const value = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(content)) as { error?: { code?: unknown } }; if (typeof value.error?.code === "string" && publicErrors.has(value.error.code)) code = value.error.code; } catch { /* Private error bodies are never forwarded. */ }
  const safeStatus = [400, 401, 403, 404, 409, 429, 503, 504].includes(status) ? status : 503;
  const response = jsonError(code, safeStatus); const retry = headers.get("retry-after"); if (retry && /^\d{1,6}$/u.test(retry)) response.headers.set("Retry-After", retry); return response;
}

export async function POST(request: NextRequest, { params }: Readonly<{ params: Promise<{ investigationId: string }> }>) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeInvestigationEvidenceBundle(request, session, rateLimit);
  if (isInvestigationSearchDenial(authorization)) return authorization;
  const investigationId = canonicalUUID((await params).investigationId);
  if (!investigationId || request.nextUrl.searchParams.size > 0) return jsonError("invalid_request", 400);
  let input: ReturnType<typeof parseWorkspaceStatusInput>;
  try { input = parseWorkspaceStatusInput(await readBoundedJSON<unknown>(request, maximumRequestBytes)); } catch { return jsonError("invalid_request", 400); }
  try {
    const connection = await privateAPIContext(authorization.session, request.headers.get("x-request-id") ?? undefined);
    const upstream = await fetch(`${connection.apiURL}/api/investigation/workspaces/${investigationId}/evidence-bundle`, { method: "POST", headers: { ...connection.headers, "Content-Type": "application/json", Accept: "application/zip" }, body: JSON.stringify(input), cache: "no-store", signal: AbortSignal.timeout(privateWriteTimeoutMilliseconds) });
    const content = await boundedBytes(upstream, upstream.ok ? maximumBundleBytes : maximumErrorBytes);
    if (!upstream.ok) return safeError(upstream.status, content, upstream.headers);
    const disposition = upstream.headers.get("content-disposition"); const sha256 = upstream.headers.get("x-ledgersync-bundle-sha256"); const expires = upstream.headers.get("x-ledgersync-bundle-expires-at");
    const filename = new RegExp(`^attachment; filename="ledgersync-investigation-${investigationId}-\\d{8}T\\d{6}Z-v1\\.zip"$`);
    if (upstream.headers.get("content-type") !== "application/zip" || upstream.headers.get("x-ledgersync-bundle-schema") !== "1" || !disposition || !filename.test(disposition) || !sha256 || !digest.test(sha256) || !expires || !timestamp.test(expires)) return jsonError("export_unavailable", 503);
    const actual = Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", content))).map((part) => part.toString(16).padStart(2, "0")).join("");
    if (actual !== sha256) return jsonError("export_unavailable", 503);
    const headers = new Headers({ "Content-Type": "application/zip", "Content-Disposition": disposition, "Content-Length": String(content.byteLength), "Cache-Control": "no-store, private, max-age=0", Pragma: "no-cache", "X-Content-Type-Options": "nosniff", "X-LedgerSync-Bundle-Schema": "1", "X-LedgerSync-Bundle-SHA256": sha256, "X-LedgerSync-Bundle-Expires-At": expires });
    const requestID = upstream.headers.get("x-request-id"); if (requestID && /^[A-Za-z0-9._:-]{1,128}$/u.test(requestID)) headers.set("X-Request-ID", requestID);
    return new Response(content, { status: 200, headers });
  } catch (error) { return jsonError(isPrivateAPITimeout(error) ? "upstream_timeout" : "export_unavailable", isPrivateAPITimeout(error) ? 504 : 503); }
}
