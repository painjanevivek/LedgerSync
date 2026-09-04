"use client";

import { useEffect } from "react";

import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { deriveConsoleCapabilities } from "@/features/console/capabilities";
import { GuideView } from "@/features/guide/GuideView";
import { LocalOrientationPanel } from "@/features/orientation/LocalOrientationPanel";
import { useOrientationWorkspace } from "@/features/orientation/useOrientationWorkspace";

export function GuideController() {
  const { session, online } = useConsoleSession();
  const capabilities = deriveConsoleCapabilities(session);
  const orientation = useOrientationWorkspace(session);
  const loadOrientation = orientation.load;

  useEffect(() => {
    if (!session || session.environment !== "local" || !online || !capabilities.localDiagnosticsRead) return;
    const timer = window.setTimeout(() => void loadOrientation(), 0);
    return () => window.clearTimeout(timer);
  }, [capabilities.localDiagnosticsRead, loadOrientation, online, session]);

  const dynamicGuide = session?.environment === "local" ? (
    <LocalOrientationPanel
      evidence={orientation.evidence}
      loading={orientation.loading}
      error={orientation.error}
      preferenceError={orientation.preferenceError}
      preferenceSaving={orientation.preferenceSaving}
      online={online}
      canRead={capabilities.localDiagnosticsRead}
      canWrite={capabilities.localOrientationWrite}
      capabilities={capabilities}
      forceOpen
      onRefresh={() => void orientation.load()}
      onUpdatePreferences={orientation.updatePreferences}
    />
  ) : null;

  return (
    <ConsoleRouteFrame section="guide" loadingLabel="Guide">
      <GuideView orientation={dynamicGuide} />
    </ConsoleRouteFrame>
  );
}
