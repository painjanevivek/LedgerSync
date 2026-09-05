"use client";

import { useEffect, useState, type ReactNode } from "react";
import { isConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { useRootEntry } from "@/features/console/WorkspaceProviders";
import { OverviewController } from "@/features/overview/OverviewController";

/** Resolve only identity before mounting any authenticated workspace providers. */
export function HomeEntry({ landing, showOrientation }: Readonly<{ landing: ReactNode; showOrientation: boolean }>) {
  const { mounted, acceptSession } = useRootEntry();
  const [error, setError] = useState(false);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    if (mounted) return;
    const controller = new AbortController();
    void fetch("/api/session", { cache: "no-store", signal: controller.signal })
      .then(async response => {
        if (response.status === 401) return null;
        if (!response.ok) throw new Error("Session unavailable");
        const value: unknown = await response.json();
        if (!isConsoleSession(value)) throw new Error("Invalid session");
        return value;
      })
      .then(value => { if (!controller.signal.aborted) { if (value) acceptSession(value); setError(false); } })
      .catch(() => { if (!controller.signal.aborted) setError(true); });
    return () => controller.abort();
  }, [attempt, mounted, acceptSession]);
  if (mounted) return <OverviewController showOrientation={showOrientation} />;
  return <>{error && <div className="public-session-notice" role="alert"><span>We couldn’t check your session. No workspace records are shown.</span><button type="button" className="button secondary" onClick={() => setAttempt(value => value + 1)}>Try again</button></div>}{landing}</>;
}
