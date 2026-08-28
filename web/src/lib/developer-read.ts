import { NextResponse } from "next/server";

import { readBoundedOperationsResponse } from "@/lib/operations-read";
import type { Session } from "@/lib/session";
import { jsonError } from "@/lib/security";
import { isPrivateAPITimeout, privateReadTimeoutMilliseconds } from "@/lib/upstream-outcome";

function safeRequestID(value: string | null) { return value && /^[A-Za-z0-9._:-]{1,128}$/.test(value) ? value : undefined; }

export function isSafeOpenAPIYAML(yaml: string) {
  return /^openapi: 3\.1\.0\r?$/m.test(yaml) && /^info:\r?$/m.test(yaml) && /^paths:\r?$/m.test(yaml) && /^components:\r?$/m.test(yaml)
    && !/-----BEGIN [A-Z ]*PRIVATE KEY-----|LEDGERSYNC_[A-Z0-9_]+\s*=|Bearer\s+[A-Za-z0-9._~-]{16,}/i.test(yaml);
}

export async function proxyDeveloperOpenAPI(session: Session): Promise<NextResponse> {
  let connection: Readonly<{ apiURL:string; headers:Record<string,string> }>;
  try {
    const { privateAPIContext } = await import("@/lib/private-api");
    connection = await privateAPIContext(session);
  } catch { return jsonError("developer_contract_unavailable", 503); }
  try {
    const upstream = await fetch(`${connection.apiURL}/api/openapi.yaml`, { headers:connection.headers,cache:"no-store",signal:AbortSignal.timeout(privateReadTimeoutMilliseconds) });
    if (!upstream.ok) {
      if (upstream.status === 401) return jsonError("unauthorized", 401);
      if (upstream.status === 403) return jsonError("forbidden", 403);
      if (upstream.status === 429) {
        const response = jsonError("rate_limited", 429);
        const retryAfter = upstream.headers.get("retry-after");
        if (retryAfter && /^[0-9]{1,6}$/.test(retryAfter)) response.headers.set("Retry-After", retryAfter);
        return response;
      }
      return jsonError("developer_contract_unavailable", 503);
    }
    const contentType = upstream.headers.get("content-type")?.split(";", 1)[0].trim().toLowerCase();
    if (!contentType || !["application/yaml", "application/x-yaml", "text/yaml"].includes(contentType)) return jsonError("developer_contract_unavailable", 503);
    const yaml = await readBoundedOperationsResponse(upstream);
    if (!isSafeOpenAPIYAML(yaml)) return jsonError("developer_contract_unavailable", 503);
    const response = new NextResponse(yaml, { status:200,headers:{ "Cache-Control":"no-store","Content-Type":"application/yaml; charset=utf-8","Content-Disposition":"attachment; filename=\"ledgersync-openapi.yaml\"","X-Content-Type-Options":"nosniff" } });
    const requestID = safeRequestID(upstream.headers.get("x-request-id"));
    if (requestID) response.headers.set("X-Request-ID", requestID);
    return response;
  } catch (error) { return jsonError(isPrivateAPITimeout(error) ? "upstream_timeout" : "developer_contract_unavailable", isPrivateAPITimeout(error) ? 504 : 503); }
}
