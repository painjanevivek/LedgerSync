import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";
import { unitStudyEvidence } from "@/lib/generated/unit-study-evidence";
import { authorizeOperationsRead, isOperationsReadDenial, strictOperationsQuery } from "@/lib/operations-read";
import { createRateLimitStore } from "@/lib/rate-limit";
import { readSession, sessionCookieName } from "@/lib/session";
const limiter = createRateLimitStore();
export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeOperationsRead(request, session, "developer:read", limiter);
  if (isOperationsReadDenial(authorization)) return authorization;
  const query = strictOperationsQuery(request, []);
  if (!(query instanceof URLSearchParams)) return query;
  return new NextResponse(unitStudyEvidence, { headers: { "Content-Type": "text/markdown; charset=utf-8", "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff", "Content-Disposition": 'attachment; filename="ledgersync-historical-study-evidence.md"' } });
}
