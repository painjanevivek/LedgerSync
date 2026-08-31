import { ApprovalsEntry } from "@/features/approvals/ApprovalsEntry";
import type { ApprovalFilters } from "@/lib/api/approvals";

function single(value: string | string[] | undefined, maximum = 255) {
  return typeof value === "string" && value.length <= maximum ? value : "";
}

function domain(value: string): ApprovalFilters["domain"] {
  return value === "funding" || value === "correction" ? value : "";
}

function age(value: string): ApprovalFilters["age"] {
  return value === "under_24h" || value === "over_24h" || value === "over_7d" || value === "over_30d" ? value : "";
}

export default async function ApprovalsPage({ searchParams }: Readonly<{ searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const query = await searchParams;
  const actionable = single(query.actionable_by_me, 5) === "true";
  return <ApprovalsEntry filters={{
    domain: domain(single(query.domain, 16)),
    status: single(query.status, 32),
    requester: single(query.requester),
    age: age(single(query.age, 16)),
    requestedAfter: single(query.requested_after, 10),
    requestedBefore: single(query.requested_before, 10),
    actionableByMe: actionable,
    cursor: single(query.cursor, 2_048) || undefined,
  }} />;
}
