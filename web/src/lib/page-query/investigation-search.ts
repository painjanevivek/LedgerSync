const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/u;
const approvedReference = /^[A-Za-z0-9][A-Za-z0-9._:/-]{7,127}$/u;

export type InvestigationSearchPageQuery = Readonly<{ query: string; queryKind: "immutable_id" | "approved_reference" | null }>;

export function parseInvestigationSearchPageQuery(searchParams: Record<string, string | string[] | undefined>): InvestigationSearchPageQuery | null {
  const keys = Object.keys(searchParams);
  if (keys.some((key) => key !== "q")) return null;
  const raw = searchParams.q;
  if (raw === undefined) return { query: "", queryKind: null };
  if (typeof raw !== "string" || raw !== raw.trim() || raw.length > 128) return null;
  const normalized = raw.toLowerCase();
  if (uuid.test(normalized)) return { query: normalized, queryKind: "immutable_id" };
  if (approvedReference.test(raw)) return { query: raw, queryKind: "approved_reference" };
  return null;
}

export function investigationSearchURL(query: string) {
  return `/search?q=${encodeURIComponent(query)}`;
}
