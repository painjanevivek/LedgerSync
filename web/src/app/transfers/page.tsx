import { OperatorConsole } from "@/features/accounts/OperatorConsole";

type SearchParams = Promise<Record<string, string | string[] | undefined>>;

export default async function TransfersPage({ searchParams }: Readonly<{ searchParams: SearchParams }>) {
  const query = await searchParams;
  const destination = typeof query.destination === "string" && query.destination.length <= 128 ? query.destination : undefined;
  const requestedReturn = typeof query.return_to === "string" ? query.return_to : undefined;
  const returnTo = requestedReturn?.startsWith("/accounts") && !requestedReturn.startsWith("//") ? requestedReturn : undefined;
  const filterQuery = typeof query.q === "string" && query.q.length <= 128 ? query.q : "";
  const filterStatus = typeof query.status === "string" && ["posted", "rejected", "pending"].includes(query.status) ? query.status : "all";
  return <OperatorConsole initialSection="transfers" initialTransferDestinationId={destination} initialTransferReturnTo={returnTo} initialTransferFilters={{ query: filterQuery, status: filterStatus }} />;
}
