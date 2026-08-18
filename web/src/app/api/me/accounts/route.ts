import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import { sessionCookieName, readSession } from "@/lib/session";
import { createActorAssertion } from "@/lib/actor-assertion";
import { jsonError } from "@/lib/security";

export async function GET() {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  if (!session) return jsonError("unauthorized", 401);
  const apiURL = process.env.LEDGERSYNC_PRIVATE_API_URL;
  const token = process.env.LEDGERSYNC_PRIVATE_API_TOKEN;
  if (!apiURL || !token) return jsonError("temporary_unavailable", 503);
  try {
    const upstream = await fetch(`${apiURL.replace(/\/$/, "")}/api/me/accounts`, { headers: { Authorization: `Bearer ${token}`, "X-LedgerSync-Actor-Assertion": createActorAssertion(session) }, cache: "no-store" });
    return new NextResponse(await upstream.text(), { status: upstream.status, headers: { "Content-Type": upstream.headers.get("content-type") ?? "application/json", "Cache-Control": "no-store" } });
  } catch { return jsonError("temporary_unavailable", 503); }
}
