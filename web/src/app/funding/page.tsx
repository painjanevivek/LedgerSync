import { FundingConsole } from "@/features/funding/FundingConsole";
import { emptyFundingFilters } from "@/lib/api/funding";
import { parseFundingPageQuery } from "@/lib/page-query/funding";

export default async function FundingPage({ searchParams }: Readonly<{ searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const parsed = parseFundingPageQuery(await searchParams);
  return <FundingConsole filters={parsed.ok ? parsed.filters : emptyFundingFilters} invalidQuery={!parsed.ok} />;
}
