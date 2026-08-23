import { OperatorConsole } from "@/features/accounts/OperatorConsole";

type SearchParams = Promise<Record<string, string | string[] | undefined>>;

function first(value: string | string[] | undefined): string { return typeof value === "string" ? value : ""; }

export default async function AccountsPage({ searchParams }: Readonly<{ searchParams: SearchParams }>) {
  const query = await searchParams;
  return <OperatorConsole initialSection="accounts" initialAccountFilters={{ query: first(query.q), status: first(query.status), category: first(query.category), cursor: first(query.cursor) || undefined }} initialAccountFocusId={first(query.focus) || undefined} />;
}
