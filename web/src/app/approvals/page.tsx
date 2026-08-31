import { ApprovalsEntry } from "@/features/approvals/ApprovalsEntry";
import { emptyApprovalFilters } from "@/lib/api/approvals";
import { parseApprovalPageQuery } from "@/lib/page-query/approvals";

export default async function ApprovalsPage({ searchParams }: Readonly<{ searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const query = await searchParams;
  const parsed = parseApprovalPageQuery(query);
  return <ApprovalsEntry filters={parsed.ok ? parsed.filters : emptyApprovalFilters} invalidQuery={!parsed.ok} />;
}
