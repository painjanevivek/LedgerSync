import { NextRequest, NextResponse } from "next/server";

import { liveSession, proxyLiveRequest, readLiveBody, validLiveId } from "@/lib/live-investigation-boundary";
import { jsonError } from "@/lib/security";

export async function GET(request: NextRequest, { params }: Readonly<{ params: Promise<{ roomId: string }> }>) {
  const authorization = await liveSession(request, false);
  if (authorization instanceof NextResponse) return authorization;
  const roomId = (await params).roomId.toLowerCase();
  const cursor = request.nextUrl.searchParams.get("cursor") ?? "0-0";
  if (!validLiveId(roomId) || request.nextUrl.searchParams.size > 1 || !/^(?:0-0|[1-9][0-9]*-[0-9]+)$/u.test(cursor)) return jsonError("invalid_request", 400);
  return proxyLiveRequest(request, authorization, `/private/investigation/live-rooms/${roomId}/signals?cursor=${encodeURIComponent(cursor)}`, "GET");
}

export async function POST(request: NextRequest, { params }: Readonly<{ params: Promise<{ roomId: string }> }>) {
  const authorization = await liveSession(request, true, false);
  if (authorization instanceof NextResponse) return authorization;
  const roomId = (await params).roomId.toLowerCase();
  if (!validLiveId(roomId) || request.nextUrl.searchParams.size > 0) return jsonError("invalid_request", 400);
  try {
    const body = await readLiveBody(request);
    if (Object.keys(body).length !== 3 || !Object.hasOwn(body, "sequence") || !Object.hasOwn(body, "kind") || !Object.hasOwn(body, "payload")
      || !Number.isSafeInteger(body.sequence) || Number(body.sequence) < 1 || !["offer", "answer", "ice", "end"].includes(String(body.kind))) return jsonError("invalid_request", 400);
    return proxyLiveRequest(request, authorization, `/private/investigation/live-rooms/${roomId}/signals`, "POST", body);
  } catch { return jsonError("invalid_request", 400); }
}
