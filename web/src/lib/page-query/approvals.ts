import type { ApprovalFilters } from "@/lib/api/approvals";
import { emptyApprovalFilters } from "@/lib/api/approvals";
import { isUTCDate, parseStrictListQuery, type StrictListQueryInput } from "@/lib/strict-list-query";

const approvalStatuses = [
  "funding:requested",
  "funding:approved",
  "funding:posted",
  "funding:rejected",
  "funding:compensated",
  "correction:requested",
  "correction:approved",
  "correction:rejected",
  "correction:cancelled",
  "correction:expired",
  "correction:posted",
] as const;

const rules = {
  domain: { maximumLength: 16, values: ["funding", "correction"] },
  status: { maximumLength: 32, values: approvalStatuses },
  requester: { maximumLength: 255, pattern: /^[A-Za-z0-9][A-Za-z0-9._:@/-]{0,254}$/ },
  age: { maximumLength: 16, values: ["under_24h", "over_24h", "over_7d", "over_30d"] },
  requested_after: { maximumLength: 10, validate: isUTCDate },
  requested_before: { maximumLength: 10, validate: isUTCDate },
  actionable_by_me: { maximumLength: 4, values: ["true"] },
  cursor: { maximumLength: 2_048 },
} as const;

export type ApprovalPageQuery =
  | Readonly<{ ok: true; filters: ApprovalFilters }>
  | Readonly<{ ok: false }>;

export function parseApprovalPageQuery(input: StrictListQueryInput): ApprovalPageQuery {
  const result = parseStrictListQuery(input, rules);
  if (!result.ok) return { ok: false };
  const values = result.values;
  if (values.domain && values.status && !values.status.startsWith(`${values.domain}:`)) return { ok: false };
  if (values.requested_after && values.requested_before && values.requested_after > values.requested_before) return { ok: false };
  return {
    ok: true,
    filters: {
      ...emptyApprovalFilters,
      domain: (values.domain ?? "") as ApprovalFilters["domain"],
      status: values.status ?? "",
      requester: values.requester ?? "",
      age: (values.age ?? "") as ApprovalFilters["age"],
      requestedAfter: values.requested_after ?? "",
      requestedBefore: values.requested_before ?? "",
      actionableByMe: values.actionable_by_me === "true",
      cursor: values.cursor,
    },
  };
}
