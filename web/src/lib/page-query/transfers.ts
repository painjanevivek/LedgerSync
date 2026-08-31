import { safeInternalReturnPath } from "@/lib/navigation";
import { parseStrictListQuery, parseStrictListSearchParams, type StrictListQueryInput, type StrictListQueryRule } from "@/lib/strict-list-query";

export type TransferFilters = Readonly<{
  query: string;
  accountId: string;
  status: "" | "pending" | "posted" | "rejected";
  from: string;
  to: string;
  cursor?: string;
}>;

export const emptyTransferFilters: TransferFilters = { query: "", accountId: "", status: "", from: "", to: "" };

const canonicalUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const transferReference = /^[0-9A-Fa-f-]{1,128}$/;
const rfc3339 = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d{1,9})?(?:Z|([+-])(\d{2}):(\d{2}))$/;

function isRFC3339(value: string) {
  const match = rfc3339.exec(value);
  if (!match) return false;
  const [, year, month, day, hour, minute, second, , offsetHour, offsetMinute] = match;
  const yearValue = Number(year);
  const monthValue = Number(month);
  const dayValue = Number(day);
  if (monthValue < 1 || monthValue > 12 || dayValue < 1 || dayValue > new Date(Date.UTC(yearValue, monthValue, 0)).getUTCDate()) return false;
  if (Number(hour) > 23 || Number(minute) > 59 || Number(second) > 59) return false;
  if (offsetHour && (Number(offsetHour) > 23 || Number(offsetMinute) > 59)) return false;
  return !Number.isNaN(Date.parse(value));
}

export const transferListQueryRules = {
  q: { maximumLength: 128, pattern: transferReference },
  accountId: { maximumLength: 36, pattern: canonicalUUID },
  status: { maximumLength: 8, values: ["pending", "posted", "rejected"] },
  from: { maximumLength: 64, validate: isRFC3339 },
  to: { maximumLength: 64, validate: isRFC3339 },
  cursor: { maximumLength: 2_048 },
} as const;

export const transferBFFQueryRules = {
  ...transferListQueryRules,
  limit: { maximumLength: 3, pattern: /^(?:[1-9]|[1-9][0-9]|100)$/ },
} as const;

export const transferExportQueryRules = {
  q: transferListQueryRules.q,
  accountId: transferListQueryRules.accountId,
  status: transferListQueryRules.status,
  from: transferListQueryRules.from,
  to: transferListQueryRules.to,
  limit: { maximumLength: 5, pattern: /^(?:[1-9][0-9]{0,3}|10000)$/ },
} as const;

const transferPageQueryRules = {
  ...transferListQueryRules,
  destination: { maximumLength: 36, pattern: canonicalUUID },
  return_to: { maximumLength: 2_048, validate: (value: string) => safeInternalReturnPath(value)?.startsWith("/accounts") === true },
} as const;

function rangeIsValid(values: Readonly<Partial<Record<string, string>>>) {
  return !(values.from && values.to && Date.parse(values.from) > Date.parse(values.to));
}

export function parseTransferPageQuery(input: StrictListQueryInput):
  | Readonly<{ ok: true; filters: TransferFilters; preferredDestinationId?: string; returnTo?: string }>
  | Readonly<{ ok: false }> {
  const result = parseStrictListQuery(input, transferPageQueryRules);
  if (!result.ok || !rangeIsValid(result.values)) return { ok: false };
  const values = result.values;
  return {
    ok: true,
    filters: {
      ...emptyTransferFilters,
      query: values.q?.toLowerCase() ?? "",
      accountId: values.accountId?.toLowerCase() ?? "",
      status: (values.status ?? "") as TransferFilters["status"],
      from: values.from ? new Date(values.from).toISOString() : "",
      to: values.to ? new Date(values.to).toISOString() : "",
      cursor: values.cursor,
    },
    preferredDestinationId: values.destination?.toLowerCase(),
    returnTo: values.return_to ? safeInternalReturnPath(values.return_to) : undefined,
  };
}

export function parseTransferSearchParams<K extends string>(searchParams: URLSearchParams, rules: Readonly<Record<K, StrictListQueryRule>>) {
  const result = parseStrictListSearchParams(searchParams, rules);
  if (!result.ok || !rangeIsValid(result.values)) return { ok: false } as const;
  return result;
}

export function transferURL(filters: TransferFilters) {
  const query = new URLSearchParams();
  if (filters.query) query.set("q", filters.query);
  if (filters.accountId) query.set("accountId", filters.accountId);
  if (filters.status) query.set("status", filters.status);
  if (filters.from) query.set("from", filters.from);
  if (filters.to) query.set("to", filters.to);
  if (filters.cursor) query.set("cursor", filters.cursor);
  return query.size ? `/transfers?${query}` : "/transfers";
}

export function transferExportQuery(filters: TransferFilters) {
  const query = new URLSearchParams({ limit: "10000" });
  if (filters.query) query.set("q", filters.query);
  if (filters.accountId) query.set("accountId", filters.accountId);
  if (filters.status) query.set("status", filters.status);
  if (filters.from) query.set("from", filters.from);
  if (filters.to) query.set("to", filters.to);
  return query;
}
