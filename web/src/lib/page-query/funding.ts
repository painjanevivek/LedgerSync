import { emptyFundingFilters, type FundingFilters } from "@/lib/api/funding";
import { parseStrictListQuery, type StrictListQueryInput } from "@/lib/strict-list-query";

export const fundingPageQueryRules = {
  status: { maximumLength: 16, values: ["requested", "approved", "posted", "rejected", "compensated"] },
  cursor: { maximumLength: 2_048 },
} as const;

export const fundingBFFQueryRules = {
  ...fundingPageQueryRules,
  limit: { maximumLength: 3, pattern: /^(?:[1-9]|[1-9][0-9]|100)$/ },
} as const;

export function parseFundingPageQuery(input: StrictListQueryInput):
  | Readonly<{ ok: true; filters: FundingFilters }>
  | Readonly<{ ok: false }> {
  const result = parseStrictListQuery(input, fundingPageQueryRules);
  if (!result.ok) return { ok: false };
  return { ok: true, filters: { ...emptyFundingFilters, status: (result.values.status ?? "") as FundingFilters["status"], cursor: result.values.cursor } };
}

export function fundingURL(filters: FundingFilters) {
  const query = new URLSearchParams();
  if (filters.status) query.set("status", filters.status);
  if (filters.cursor) query.set("cursor", filters.cursor);
  return query.size ? `/funding?${query}` : "/funding";
}
