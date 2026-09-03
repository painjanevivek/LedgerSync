import { emptyCorrectionFilters, type CorrectionFilters } from "@/lib/api/corrections";
import { parseStrictListQuery, type StrictListQueryInput } from "@/lib/strict-list-query";

export const correctionPageQueryRules = {
  status: { maximumLength: 16, values: ["requested", "approved", "posted", "rejected", "cancelled", "expired"] },
  cursor: { maximumLength: 2_048 },
} as const;

export const correctionBFFQueryRules = {
  ...correctionPageQueryRules,
  limit: { maximumLength: 3, pattern: /^(?:[1-9]|[1-9][0-9]|100)$/ },
} as const;

export function parseCorrectionPageQuery(input: StrictListQueryInput):
  | Readonly<{ ok: true; filters: CorrectionFilters }>
  | Readonly<{ ok: false }> {
  const result = parseStrictListQuery(input, correctionPageQueryRules);
  if (!result.ok) return { ok: false };
  return {
    ok: true,
    filters: {
      ...emptyCorrectionFilters,
      status: (result.values.status ?? "") as CorrectionFilters["status"],
      cursor: result.values.cursor,
    },
  };
}

export function correctionsURL(filters: CorrectionFilters) {
  const query = new URLSearchParams();
  if (filters.status) query.set("status", filters.status);
  if (filters.cursor) query.set("cursor", filters.cursor);
  return query.size ? `/corrections?${query}` : "/corrections";
}
