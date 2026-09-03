import { parseStrictListQuery, parseStrictListSearchParams, type StrictListQueryInput } from "@/lib/strict-list-query";
import { canonicalUUID } from "@/lib/canonical-uuid";

export type AccountStatusFilter = "" | "active" | "frozen" | "closed";
export type AccountCategoryFilter = "" | "operating" | "customer_funds" | "payroll" | "payables" | "expenses" | "reserve";
export type AccountFilters = Readonly<{
  query: string;
  status: AccountStatusFilter;
  category: AccountCategoryFilter;
  cursor?: string;
}>;

export const emptyAccountFilters: AccountFilters = { query: "", status: "", category: "" };

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const controlCharacters = /[\u0000-\u001f\u007f-\u009f]/u;

export function isAccountId(value: string) {
  return canonicalUUID(value) !== undefined;
}

function isSafeAccountSearch(value: string) {
  return !controlCharacters.test(value);
}

export const accountPageQueryRules = {
  q: { maximumLength: 128, validate: isSafeAccountSearch },
  status: { maximumLength: 16, values: ["active", "frozen", "closed"] },
  category: { maximumLength: 32, values: ["operating", "customer_funds", "payroll", "payables", "expenses", "reserve"] },
  cursor: { maximumLength: 512 },
  focus: { maximumLength: 36, pattern: uuid },
} as const;

export const accountBFFQueryRules = {
  q: accountPageQueryRules.q,
  status: accountPageQueryRules.status,
  category: accountPageQueryRules.category,
  cursor: accountPageQueryRules.cursor,
  limit: { maximumLength: 3, pattern: /^(?:[1-9]|[1-9][0-9]|100)$/ },
} as const;

export const accountHistoryBFFQueryRules = {
  cursor: { maximumLength: 512 },
  limit: { maximumLength: 3, pattern: /^(?:[1-9]|[1-9][0-9]|100)$/ },
} as const;

export function parseAccountPageQuery(input: StrictListQueryInput): Readonly<{ ok: true; filters: AccountFilters; focusAccountId?: string }> | Readonly<{ ok: false }> {
  const parsed = parseStrictListQuery(input, accountPageQueryRules);
  if (!parsed.ok) return { ok: false };
  return {
    ok: true,
    filters: {
      query: parsed.values.q ?? "",
      status: (parsed.values.status ?? "") as AccountStatusFilter,
      category: (parsed.values.category ?? "") as AccountCategoryFilter,
      cursor: parsed.values.cursor,
    },
    focusAccountId: parsed.values.focus?.toLowerCase(),
  };
}

export function parseAccountBFFSearchParams(searchParams: URLSearchParams) {
  return parseStrictListSearchParams(searchParams, accountBFFQueryRules);
}

export function parseAccountHistoryBFFSearchParams(searchParams: URLSearchParams) {
  return parseStrictListSearchParams(searchParams, accountHistoryBFFQueryRules);
}

export function accountDirectoryURL(filters: AccountFilters, focusAccountId?: string): string {
  const query = new URLSearchParams();
  if (filters.query) query.set("q", filters.query);
  if (filters.status) query.set("status", filters.status);
  if (filters.category) query.set("category", filters.category);
  if (filters.cursor) query.set("cursor", filters.cursor);
  if (focusAccountId) query.set("focus", focusAccountId);
  return query.size ? `/accounts?${query}` : "/accounts";
}

export function accountDetailURL(accountId: string, filters: AccountFilters): string {
  const returnTo = accountDirectoryURL(filters, accountId);
  return `/accounts/${encodeURIComponent(accountId)}?return_to=${encodeURIComponent(returnTo)}`;
}

export function accountFiltersFromReturnPath(returnTo: string): AccountFilters {
  try {
    const url = new URL(returnTo, "https://ledgersync.invalid");
    if (url.pathname !== "/accounts") return emptyAccountFilters;
    const parsed = parseStrictListSearchParams(url.searchParams, accountPageQueryRules);
    if (!parsed.ok) return emptyAccountFilters;
    return {
      query: parsed.values.q ?? "",
      status: (parsed.values.status ?? "") as AccountStatusFilter,
      category: (parsed.values.category ?? "") as AccountCategoryFilter,
      cursor: parsed.values.cursor,
    };
  } catch {
    return emptyAccountFilters;
  }
}
