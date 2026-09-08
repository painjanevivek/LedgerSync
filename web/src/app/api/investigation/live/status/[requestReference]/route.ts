import { NextRequest, NextResponse } from "next/server";

import { liveSession, proxyLiveRequest, validLiveId } from "@/lib/live-investigation-boundary";

export async function GET(request: NextRequest, { params }: Readonly<{ params: Promise<{ requestReference: string }> }>) {
  const authorization = await liveSession(request, false);
  if (authorization instanceof NextResponse) return authorization;
  const reference = (await params).requestReference.toLowerCase();
  if (!validLiveId(reference) || request.nextUrl.searchParams.size > 0) return NextResponse.json({ error: { code: "invalid_request" } }, { status: 400 });
  return proxyLiveRequest(request, authorization, `/private/transfer-requests/${reference}/status`, "GET");
}
