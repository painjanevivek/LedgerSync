import Link from "next/link";

export type ActiveFilter = Readonly<{ label: string; value: string }>;

export function ActiveFilterSummary({
  filters,
  clearHref,
}: Readonly<{ filters: readonly ActiveFilter[]; clearHref: string }>) {
  if (filters.length === 0) return null;
  return (
    <section className="active-filter-summary" aria-label="Active filters">
      <strong>Filtered by</strong>
      <ul>
        {filters.map((filter) => (
          <li key={`${filter.label}:${filter.value}`}>
            <span>{filter.label}</span>
            <b>{filter.value}</b>
          </li>
        ))}
      </ul>
      <Link className="text-link" href={clearHref}>
        Clear all filters
      </Link>
    </section>
  );
}
