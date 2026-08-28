import { FundingConsole } from "@/features/funding/FundingConsole";

export default async function FundingDetailPage({ params }: Readonly<{ params: Promise<{ fundingEventId: string }> }>) {
  const { fundingEventId } = await params;
  return <FundingConsole fundingEventId={fundingEventId} />;
}
