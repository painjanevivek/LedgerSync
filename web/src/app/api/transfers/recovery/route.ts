import { cookies } from "next/headers";
import { NextRequest } from "next/server";
import { readSession, sessionCookieName } from "@/lib/session";
import { hasValidCSRF, jsonError, readBoundedJSON } from "@/lib/security";
import { proxyPrivateGET } from "@/lib/private-api";
import { toPrivateTransferRequest, type CreateTransferInput } from "@/lib/api/transfers";
import { canonicalUUID } from "@/lib/canonical-uuid";
import { proxyTransferMutation } from "@/lib/transfer-mutation";

export async function GET(request: NextRequest) {
 const session=readSession((await cookies()).get(sessionCookieName)?.value);
 if(!session) return jsonError("unauthorized",401);
 if(!session.scopes?.includes("transfers:read")) return jsonError("forbidden",403);
 return proxyPrivateGET(request,session,"/api/transfers/recovery",["cursor"]);
}

export async function POST(request: NextRequest) {
 const session=readSession((await cookies()).get(sessionCookieName)?.value);
 if(!session) return jsonError("unauthorized",401);
 if(!hasValidCSRF(request,session)) return jsonError("csrf_failed",403);
 if(!session.scopes?.includes("transfers:write")) return jsonError("forbidden",403);
 if(request.nextUrl.searchParams.size) return jsonError("invalid_request",400);
 const key=request.headers.get("idempotency-key")?.trim();
 const reference=canonicalUUID(request.headers.get("x-ledgersync-request-reference")??"");
 if(!key || !reference) return jsonError("validation_failed",400);
 try {
  const body=toPrivateTransferRequest(await readBoundedJSON<CreateTransferInput>(request));
  return proxyTransferMutation(session,"/api/transfers/recovery",body,key,reference);
 } catch {return jsonError("validation_failed",400);}
}
