import { AccountsController } from "@/features/accounts/AccountsController";
import { accountDirectoryURL, emptyAccountFilters, parseAccountPageQuery } from "@/lib/page-query/accounts";

type SearchParams = Promise<Record<string, string | string[] | undefined>>;

export default async function AccountsPage({ searchParams }: Readonly<{ searchParams: SearchParams }>) {
  const query = await searchParams;
  const parsed = parseAccountPageQuery(query);
  const filters = parsed.ok ? parsed.filters : emptyAccountFilters;
  const focusAccountId = parsed.ok ? parsed.focusAccountId : undefined;
  return <AccountsController key={parsed.ok ? accountDirectoryURL(filters, focusAccountId) : "invalid-account-query"} filters={filters} focusAccountId={focusAccountId} invalidQuery={!parsed.ok} />;
}
