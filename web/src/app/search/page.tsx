import { InvestigationSearchController } from "@/features/investigation/InvestigationSearchController";
import { parseInvestigationSearchPageQuery } from "@/lib/page-query/investigation-search";

export default async function SearchPage({ searchParams }: Readonly<{ searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const parsed = parseInvestigationSearchPageQuery(await searchParams);
  const initialQuery = parsed?.query ?? "";
  return <InvestigationSearchController key={`${initialQuery}:${parsed === null ? "invalid" : "valid"}`} initialQuery={initialQuery} invalidQuery={parsed === null} />;
}
