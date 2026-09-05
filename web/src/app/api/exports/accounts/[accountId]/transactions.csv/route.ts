import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { authorizeEvidenceExport, isEvidenceExportDenial, proxyEvidenceExport, strictExportQuery } from "@/lib/export-read";
import { jsonError } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";
import { canonicalUUID } from "@/lib/canonical-uuid";

export async function GET(request: NextRequest, context: { params: Promise<{ accountId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeEvidenceExport(request, session, "transactions:read"); if (isEvidenceExportDenial(authorization)) return authorization;
  const accountId = canonicalUUID((await context.params).accountId);
  if (!accountId) return jsonError("validation_failed", 400);
  const query = strictExportQuery(request, "account"); if (!(query instanceof URLSearchParams)) return query;
  return proxyEvidenceExport(authorization, `/api/exports/accounts/${encodeURIComponent(accountId)}/transactions.csv`, query, "account-ledger");
}
