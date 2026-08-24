import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { authorizeEvidenceExport, isEvidenceExportDenial, proxyEvidenceExport, strictExportQuery } from "@/lib/export-read";
import { readSession, sessionCookieName } from "@/lib/session";

export async function GET(request: NextRequest) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeEvidenceExport(request, session, "reconciliation:read"); if (isEvidenceExportDenial(authorization)) return authorization;
  const query = strictExportQuery(request, "reconciliation"); if (!(query instanceof URLSearchParams)) return query;
  return proxyEvidenceExport(authorization, "/api/exports/reconciliation.csv", query, "reconciliation");
}
