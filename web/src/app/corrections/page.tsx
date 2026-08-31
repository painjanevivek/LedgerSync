import { CorrectionsConsole } from "@/features/corrections/CorrectionsConsole";
import { emptyCorrectionFilters } from "@/lib/api/corrections";
import { parseCorrectionPageQuery } from "@/lib/page-query/corrections";

export default async function CorrectionsPage({ searchParams }: Readonly<{ searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const parsed = parseCorrectionPageQuery(await searchParams);
  return <CorrectionsConsole filters={parsed.ok ? parsed.filters : emptyCorrectionFilters} invalidQuery={!parsed.ok} />;
}
