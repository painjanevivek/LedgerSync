import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { LiveInvestigationJoin } from "@/features/investigation/LiveInvestigationJoin";

export default async function LiveInvestigationRoomPage({ params }: Readonly<{ params: Promise<{ roomId: string }> }>) {
  const { roomId } = await params;
  return <ConsoleRouteFrame section="search" loadingLabel="Live investigation"><LiveInvestigationJoin roomId={roomId.toLowerCase()} /></ConsoleRouteFrame>;
}
