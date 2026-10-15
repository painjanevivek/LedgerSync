import { NextRequest, NextResponse } from "next/server";

import { liveSession, proxyLiveRequest, readLiveBody, validLiveId } from "@/lib/live-investigation-boundary";
import { jsonError } from "@/lib/security";

export async function POST(request: NextRequest, { params }: Readonly<{ params: Promise<{ roomId: string }> }>) {
  const authorization = await liveSession(request, true, false);
  if (authorization instanceof NextResponse) return authorization;
  const roomId = (await params).roomId.toLowerCase();
  if (!validLiveId(roomId) || request.nextUrl.searchParams.size > 0) return jsonError("invalid_request", 400);
  try {
    const body = await readLiveBody(request, 512);
    if (Object.keys(body).length !== 0) return jsonError("invalid_request", 400);
    return proxyLiveRequest(request, authorization, `/private/investigation/live-rooms/${roomId}/presence`, "POST", body);
  } catch { return jsonError("invalid_request", 400); }
}
