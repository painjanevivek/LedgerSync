import { OperationsConsole } from "@/features/operations/OperationsConsole";
import { emptyEventFilters, parseEventPageQuery } from "@/lib/page-query/operations";

export default async function EventsPage({ searchParams }: Readonly<{ searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const query = await searchParams;
  const parsed = parseEventPageQuery(query);
  return <OperationsConsole section="events" filters={parsed.ok ? parsed.filters : emptyEventFilters} invalidQuery={!parsed.ok} />;
}
