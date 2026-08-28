import { CorrectionsConsole } from "@/features/corrections/CorrectionsConsole";

export default async function CorrectionDetailPage({
  params,
}: Readonly<{ params: Promise<{ correctionId: string }> }>) {
  const { correctionId } = await params;
  return <CorrectionsConsole correctionId={correctionId} />;
}
