"use client";

import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { LiveInvestigationJoin } from "@/features/investigation/LiveInvestigationJoin";

export default function LiveInvestigationPage() {
  return <ConsoleRouteFrame section="search" loadingLabel="Live investigation"><LiveInvestigationJoin /></ConsoleRouteFrame>;
}
