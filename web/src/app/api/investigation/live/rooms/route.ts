import { NextRequest, NextResponse } from "next/server";

import { liveSession, proxyLiveRequest, readLiveBody, validIdempotencyKey, validLiveId } from "@/lib/live-investigation-boundary";
import { jsonError } from "@/lib/security";

export async function POST(request: NextRequest) {
  const authorization = await liveSession(request, true, true);
  if (authorization instanceof NextResponse) return authorization;
  if (!validIdempotencyKey(request) || request.nextUrl.searchParams.size > 0) return jsonError("invalid_request", 400);
  try {
    const body = await readLiveBody(request, 4096);
    if (Object.keys(body).length !== 1 || !validLiveId(body.request_reference as string)) return jsonError("invalid_request", 400);
    return proxyLiveRequest(request, authorization, "/private/investigation/live-rooms", "POST", body);
  } catch { return jsonError("invalid_request", 400); }
}
