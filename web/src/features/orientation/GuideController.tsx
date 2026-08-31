"use client";

import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { GuideView } from "@/features/guide/GuideView";

export function GuideController() {
  return (
    <ConsoleRouteFrame section="guide" loadingLabel="Guide">
      <GuideView />
    </ConsoleRouteFrame>
  );
}
