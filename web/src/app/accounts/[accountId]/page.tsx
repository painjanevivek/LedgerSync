import { OperatorConsole } from "@/features/accounts/OperatorConsole";

export default async function AccountPage({ params, searchParams }: { params: Promise<{ accountId: string }>; searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const { accountId } = await params;
  const query = await searchParams;
  const requested = typeof query.return_to === "string" ? query.return_to : "/accounts";
  const returnTo = requested.startsWith("/") && !requested.startsWith("//") && requested.length <= 1024 ? requested : "/accounts";
  const restored = returnTo.startsWith("/accounts") ? new URL(returnTo, "http://ledgersync.local").searchParams : new URLSearchParams();
  return <OperatorConsole initialSection="accounts" initialAccountId={accountId} initialAccountFilters={{ query: restored.get("q") ?? "", status: restored.get("status") ?? "", category: restored.get("category") ?? "", cursor: restored.get("cursor") ?? undefined }} initialAccountFocusId={accountId} initialAccountReturnTo={returnTo} />;
}
