import { CorrectionsConsole } from "@/features/corrections/CorrectionsConsole";
import { safeInternalReturnPath } from "@/lib/navigation";

export default async function CorrectionDetailPage({
  params,
  searchParams,
}: Readonly<{ params: Promise<{ correctionId: string }>; searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const { correctionId } = await params;
  const query = await searchParams;
  return <CorrectionsConsole correctionId={correctionId} detailReturnTo={safeInternalReturnPath(query.return_to)} />;
}
