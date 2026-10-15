import { NextRequest, NextResponse } from "next/server";
import { GET as readAuthorizedEvent } from "../route";
import { jsonError } from "@/lib/security";
import { traceViewerURL } from "@/lib/trace-viewer";

export async function GET(request: NextRequest, context: { params: Promise<{ eventId: string }> }) {
  // Recheck the current record authority. A saved URL is never a capability.
  const evidence = await readAuthorizedEvent(request, context);
  if (!evidence.ok) return evidence;
  const record = await evidence.json();
  const target = traceViewerURL(record.diagnostic_trace_id, process.env.LEDGERSYNC_TRACE_VIEWER_BASE_URL);
  if (!target) return jsonError("trace_viewer_unavailable", 503);
  const response = NextResponse.redirect(target, 307);
  response.headers.set("Cache-Control", "no-store");
  response.headers.set("Referrer-Policy", "no-referrer");
  return response;
}
