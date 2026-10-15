import { NextRequest, NextResponse } from "next/server";

import { liveSession, proxyLiveRequest, readLiveBody, validIdempotencyKey } from "@/lib/live-investigation-boundary";
import { jsonError } from "@/lib/security";

export async function POST(request: NextRequest) {
  const authorization = await liveSession(request, true, false);
  if (authorization instanceof NextResponse) return authorization;
  if (!validIdempotencyKey(request) || request.nextUrl.searchParams.size > 0) return jsonError("invalid_request", 400);
  try {
    const body = await readLiveBody(request, 4096);
    if (Object.keys(body).length !== 1 || typeof body.invite_token !== "string" || body.invite_token.length < 32 || body.invite_token.length > 128) return jsonError("invalid_request", 400);
    return proxyLiveRequest(request, authorization, "/private/investigation/live-rooms/join", "POST", body);
  } catch { return jsonError("invalid_request", 400); }
}
