import { OperatorConsole } from "@/features/accounts/OperatorConsole";

export default async function TransferPage({ params }: { params: Promise<{ transferId: string }> }) {
  const { transferId } = await params;
  return <OperatorConsole initialSection="transfers" initialTransferId={transferId} />;
}
