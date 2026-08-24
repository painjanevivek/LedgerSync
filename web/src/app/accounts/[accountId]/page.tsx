import { OperatorConsole } from "@/features/accounts/OperatorConsole";

export default async function AccountPage({ params, searchParams }: { params: Promise<{ accountId: string }>; searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const { accountId } = await params;
  const query = await searchParams;
  const returnTo = typeof query.return_to === "string" && query.return_to.startsWith("/accounts") ? query.return_to : "/accounts";
  const restored = new URL(returnTo, "http://ledgersync.local").searchParams;
  return <OperatorConsole initialSection="accounts" initialAccountId={accountId} initialAccountFilters={{ query: restored.get("q") ?? "", status: restored.get("status") ?? "", category: restored.get("category") ?? "", cursor: restored.get("cursor") ?? undefined }} initialAccountFocusId={accountId} />;
}
