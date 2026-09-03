import { FundingConsole } from "@/features/funding/FundingConsole";
import { safeInternalReturnPath } from "@/lib/navigation";

export default async function FundingDetailPage({ params, searchParams }: Readonly<{ params: Promise<{ fundingEventId: string }>; searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const { fundingEventId } = await params;
  const query = await searchParams;
  return <FundingConsole fundingEventId={fundingEventId} detailReturnTo={safeInternalReturnPath(query.return_to)} />;
}
