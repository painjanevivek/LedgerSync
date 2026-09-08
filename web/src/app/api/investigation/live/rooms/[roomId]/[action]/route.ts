import { NextRequest, NextResponse } from "next/server";

import { liveSession, proxyLiveRequest, readLiveBody, validIdempotencyKey, validLiveId } from "@/lib/live-investigation-boundary";
import { jsonError } from "@/lib/security";

const actions = new Set(["invite", "leave", "end", "finding"]);

export async function POST(request: NextRequest, { params }: Readonly<{ params: Promise<{ roomId: string; action: string }> }>) {
  const route = await params;
  const roomId = route.roomId.toLowerCase();
  if (!validLiveId(roomId) || !actions.has(route.action) || request.nextUrl.searchParams.size > 0) return jsonError("invalid_request", 400);
  const requireWrite = route.action === "invite" || route.action === "end" || route.action === "finding";
  const authorization = await liveSession(request, true, requireWrite);
  if (authorization instanceof NextResponse) return authorization;
  if ((route.action === "end" || route.action === "finding") && !validIdempotencyKey(request)) return jsonError("invalid_request", 400);
  try {
    const body = await readLiveBody(request, 4096);
    if (route.action === "invite") {
      if (Object.keys(body).length !== 0) return jsonError("invalid_request", 400);
    } else {
      const expectedKeys = route.action === "finding" ? ["expected_version", "code"] : ["expected_version"];
      if (Object.keys(body).length !== expectedKeys.length || !expectedKeys.every((key) => Object.hasOwn(body, key))
        || typeof body.expected_version !== "string" || !/^[1-9][0-9]{0,18}$/u.test(body.expected_version)) return jsonError("invalid_request", 400);
      if (route.action === "finding" && !["confirmed_completed", "confirmed_rejected", "still_unresolved", "escalated"].includes(String(body.code))) return jsonError("invalid_request", 400);
    }
    return proxyLiveRequest(request, authorization, `/private/investigation/live-rooms/${roomId}/${route.action}`, "POST", body);
  } catch { return jsonError("invalid_request", 400); }
}
