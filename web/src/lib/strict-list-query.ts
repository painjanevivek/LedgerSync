export type StrictListQueryInput = Readonly<Record<string, string | string[] | undefined>>;

export type StrictListQueryRule = Readonly<{
  maximumLength?: number;
  values?: readonly string[];
  pattern?: RegExp;
  validate?: (value: string) => boolean;
}>;

export type StrictListQueryResult<K extends string> =
  | Readonly<{ ok: true; values: Partial<Record<K, string>> }>
  | Readonly<{ ok: false; reason: "unknown" | "duplicate" | "empty" | "oversized" | "malformed" }>;

/**
 * Parse a server-page query without silently widening or changing its scope.
 * The BFF and private API still validate independently; this prevents an
 * invalid shared URL from starting a protected browser request at all.
 */
export function parseStrictListQuery<K extends string>(
  input: StrictListQueryInput,
  rules: Readonly<Record<K, StrictListQueryRule>>,
): StrictListQueryResult<K> {
  const values: Partial<Record<K, string>> = {};
  for (const [key, raw] of Object.entries(input)) {
    if (raw === undefined) continue;
    if (!Object.prototype.hasOwnProperty.call(rules, key)) return { ok: false, reason: "unknown" };
    if (Array.isArray(raw)) return { ok: false, reason: "duplicate" };

    const rule = rules[key as K];
    const value = raw.trim();
    if (!value) return { ok: false, reason: "empty" };
    if (value.length > (rule.maximumLength ?? 256)) return { ok: false, reason: "oversized" };
    if (rule.values && !rule.values.includes(value)) return { ok: false, reason: "malformed" };
    if (rule.pattern) {
      rule.pattern.lastIndex = 0;
      if (!rule.pattern.test(value)) return { ok: false, reason: "malformed" };
    }
    if (rule.validate && !rule.validate(value)) return { ok: false, reason: "malformed" };
    values[key as K] = value;
  }
  return { ok: true, values };
}

export function parseStrictListSearchParams<K extends string>(
  searchParams: Pick<URLSearchParams, "keys" | "getAll">,
  rules: Readonly<Record<K, StrictListQueryRule>>,
): StrictListQueryResult<K> {
  const input: Record<string, string | string[]> = {};
  for (const key of new Set(searchParams.keys())) {
    const values = searchParams.getAll(key);
    input[key] = values.length === 1 ? values[0] : values;
  }
  return parseStrictListQuery(input, rules);
}

export function isUTCDate(value: string) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
  const date = new Date(`${value}T00:00:00.000Z`);
  return !Number.isNaN(date.valueOf()) && date.toISOString().slice(0, 10) === value;
}

const rfc3339 = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d{1,9})?(?:Z|([+-])(\d{2}):(\d{2}))$/;

export function isRFC3339Timestamp(value: string) {
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
