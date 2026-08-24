import { OperatorConsole } from "@/features/accounts/OperatorConsole";

export default async function TransferPage({ params, searchParams }: { params: Promise<{ transferId: string }>; searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const { transferId } = await params;
  const query = await searchParams;
  const requested = typeof query.return_to === "string" ? query.return_to : "/transfers";
  const returnTo = requested.startsWith("/") && !requested.startsWith("//") && requested.length <= 1024 ? requested : "/transfers";
  return <OperatorConsole initialSection="transfers" initialTransferId={transferId} initialTransferReturnTo={returnTo} />;
}
