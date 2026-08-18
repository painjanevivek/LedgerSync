import { OperatorConsole } from "@/features/accounts/OperatorConsole";

export default async function AccountPage({ params }: { params: Promise<{ accountId: string }> }) {
  const { accountId } = await params;
  return <OperatorConsole initialSection="accounts" initialAccountId={accountId} />;
}
