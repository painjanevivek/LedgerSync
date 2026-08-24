import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import { authorizeEvidenceExport, isEvidenceExportDenial, proxyEvidenceExport, strictExportQuery } from "@/lib/export-read";
import { jsonError } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";

export async function GET(request: NextRequest, context: { params: Promise<{ accountId: string }> }) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = await authorizeEvidenceExport(request, session, "transactions:read"); if (isEvidenceExportDenial(authorization)) return authorization;
  const { accountId } = await context.params; if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(accountId)) return jsonError("not_found", 404);
  const query = strictExportQuery(request, "account"); if (!(query instanceof URLSearchParams)) return query;
  return proxyEvidenceExport(authorization, `/api/exports/accounts/${encodeURIComponent(accountId)}/transactions.csv`, query, "account-ledger");
}
