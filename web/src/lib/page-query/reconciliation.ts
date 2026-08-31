import { parseStrictListQuery, type StrictListQueryInput } from "@/lib/strict-list-query";

export type ReconciliationFilters = Readonly<{ cursor?: string }>;
export const emptyReconciliationFilters: ReconciliationFilters = {};

export const reconciliationPageQueryRules = {
  cursor: { maximumLength: 2_048 },
} as const;

export const reconciliationBFFQueryRules = {
  ...reconciliationPageQueryRules,
  limit: { maximumLength: 3, pattern: /^(?:[1-9]|[1-9][0-9]|100)$/ },
} as const;

export function parseReconciliationPageQuery(input: StrictListQueryInput):
  | Readonly<{ ok: true; filters: ReconciliationFilters }>
  | Readonly<{ ok: false }> {
  const result = parseStrictListQuery(input, reconciliationPageQueryRules);
  if (!result.ok) return { ok: false };
  return { ok: true, filters: { cursor: result.values.cursor } };
}

export function reconciliationURL(filters: ReconciliationFilters) {
  const query = new URLSearchParams();
  if (filters.cursor) query.set("cursor", filters.cursor);
  return query.size ? `/reconciliation?${query}` : "/reconciliation";
}
