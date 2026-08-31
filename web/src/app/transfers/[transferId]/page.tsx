import { TransfersController } from "@/features/transfers/TransfersController";
import { safeInternalReturnPath } from "@/lib/navigation";

export default async function TransferPage({ params, searchParams }: { params: Promise<{ transferId: string }>; searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const { transferId } = await params;
  const query = await searchParams;
  const returnTo = safeInternalReturnPath(query.return_to) ?? "/transfers";
  return <TransfersController transferId={transferId} returnTo={returnTo} />;
}
