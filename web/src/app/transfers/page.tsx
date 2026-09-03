import { TransfersController } from "@/features/transfers/TransfersController";
import { emptyTransferFilters, parseTransferPageQuery } from "@/lib/page-query/transfers";

type SearchParams = Promise<Record<string, string | string[] | undefined>>;

export default async function TransfersPage({ searchParams }: Readonly<{ searchParams: SearchParams }>) {
  const query = await searchParams;
  const parsed = parseTransferPageQuery(query);
  return <TransfersController preferredDestinationId={parsed.ok ? parsed.preferredDestinationId : undefined} returnTo={parsed.ok ? parsed.returnTo : undefined} filters={parsed.ok ? parsed.filters : emptyTransferFilters} invalidQuery={!parsed.ok} />;
}
