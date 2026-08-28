import { OperatorConsole } from "@/features/accounts/OperatorConsole";

type SearchParams = Promise<Record<string, string | string[] | undefined>>;

export default async function NewAccountPage({ searchParams }: Readonly<{ searchParams: SearchParams }>) {
  const query = await searchParams;
  const requested = typeof query.return_to === "string" ? query.return_to : "/accounts";
  const returnTo = requested.startsWith("/accounts") && !requested.startsWith("//") ? requested : "/accounts";
  return <OperatorConsole initialSection="accounts" initialAccountCreate initialAccountReturnTo={returnTo} />;
}
