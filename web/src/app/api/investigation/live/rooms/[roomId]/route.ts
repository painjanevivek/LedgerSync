import { NextRequest, NextResponse } from "next/server";

import { liveSession, proxyLiveRequest, validLiveId } from "@/lib/live-investigation-boundary";
import { jsonError } from "@/lib/security";

export async function GET(request: NextRequest, { params }: Readonly<{ params: Promise<{ roomId: string }> }>) {
  const authorization = await liveSession(request, false);
  if (authorization instanceof NextResponse) return authorization;
  const roomId = (await params).roomId.toLowerCase();
  if (!validLiveId(roomId) || request.nextUrl.searchParams.size > 0) return jsonError("invalid_request", 400);
  return proxyLiveRequest(request, authorization, `/private/investigation/live-rooms/${roomId}`, "GET");
}
