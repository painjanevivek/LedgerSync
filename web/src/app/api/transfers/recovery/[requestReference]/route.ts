import { cookies } from "next/headers";
import { NextRequest } from "next/server";
import { readSession, sessionCookieName } from "@/lib/session";
import { hasValidCSRF, jsonError } from "@/lib/security";
import { canonicalUUID } from "@/lib/canonical-uuid";
import { proxyTransferMutation } from "@/lib/transfer-mutation";

export async function POST(request: NextRequest, context: {params: Promise<{requestReference: string}>}) {
 const session=readSession((await cookies()).get(sessionCookieName)?.value);
 if(!session) return jsonError("unauthorized",401);
 if(!hasValidCSRF(request,session)) return jsonError("csrf_failed",403);
 if(!session.scopes?.includes("transfers:write")) return jsonError("forbidden",403);
 const reference=canonicalUUID((await context.params).requestReference);
 if(!reference || request.nextUrl.searchParams.size) return jsonError("invalid_request",400);
 // An empty body is required. No replacement financial details can cross this boundary.
 if(request.body) {
  const reader=request.body.getReader();
  try { while(true) {const chunk=await reader.read();if(chunk.done) break;if(chunk.value.length) return jsonError("validation_failed",400);} }
  finally {await reader.cancel();}
 }
 return proxyTransferMutation(session,`/api/transfers/recovery/${reference}/retry`,undefined,undefined,reference,request.headers.get("x-request-id")??undefined,request.headers.get("x-ledgersync-session-handle")??undefined);
}
