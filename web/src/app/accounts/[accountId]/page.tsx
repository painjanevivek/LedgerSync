import { notFound } from "next/navigation";

import { AccountsController } from "@/features/accounts/AccountsController";
import { safeInternalReturnPath } from "@/lib/navigation";
import { accountFiltersFromReturnPath, isAccountId } from "@/lib/page-query/accounts";

export default async function AccountPage({ params, searchParams }: { params: Promise<{ accountId: string }>; searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const { accountId } = await params;
  if (!isAccountId(accountId)) notFound();
  const query = await searchParams;
  const returnTo = safeInternalReturnPath(query.return_to) ?? "/accounts";
  const normalizedAccountId = accountId.toLowerCase();
  return <AccountsController key={normalizedAccountId} accountId={normalizedAccountId} filters={accountFiltersFromReturnPath(returnTo)} focusAccountId={normalizedAccountId} returnTo={returnTo} />;
}
