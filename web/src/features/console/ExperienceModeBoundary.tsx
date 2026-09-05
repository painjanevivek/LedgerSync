"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import type { ExperienceMode } from "@/features/console/experience-mode";
import { sanitizeUIPreference } from "@/lib/api/ui-preferences";

type ExperienceModeContextValue = Readonly<{
  mode: ExperienceMode;
  setMode: (mode: ExperienceMode) => void;
}>;

const ExperienceModeContext = createContext<ExperienceModeContextValue | null>(null);

export function ExperienceModeProvider({
  children,
  mode,
  setMode,
}: Readonly<ExperienceModeContextValue & { children?: ReactNode }>) {
  const value = useMemo<ExperienceModeContextValue>(() => ({ mode, setMode }), [mode, setMode]);
  return <ExperienceModeContext.Provider value={value}>{children}</ExperienceModeContext.Provider>;
}

export function ExperienceModeBoundary({ children }: Readonly<{ children: ReactNode }>) {
  const { session } = useConsoleSession();
  const [mode, setModeState] = useState<ExperienceMode>("simple");
  const version = useRef("0");

  useEffect(() => {
    let active = true;
    version.current = "0";
    if (!session) {
      const timer = window.setTimeout(() => { if (active) setModeState("simple"); }, 0);
      return () => { active = false; window.clearTimeout(timer); };
    }
    void fetch("/api/preferences", { cache: "no-store" })
      .then(async (response) => response.ok ? sanitizeUIPreference(await response.json()) : null)
      .then((preference) => {
        if (!active) return;
        if (preference) {
          version.current = preference.version;
          setModeState(preference.experience_mode);
        } else setModeState("simple");
      })
      .catch(() => { if (active) setModeState("simple"); });
    return () => { active = false; };
  }, [session]);

  useEffect(() => {
    document.documentElement.dataset.experienceMode = mode;
    return () => { delete document.documentElement.dataset.experienceMode; };
  }, [mode]);

  const setMode = useCallback((nextMode: ExperienceMode) => {
    setModeState(nextMode);
    if (!session) return;
    void fetch("/api/preferences", {
      method: "PATCH",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": session.csrf_token },
      body: JSON.stringify({ experience_mode: nextMode, expected_version: version.current }),
    }).then(async (response) => response.ok ? sanitizeUIPreference(await response.json()) : null)
      .then((preference) => {
        if (preference) {
          version.current = preference.version;
          setModeState(preference.experience_mode);
        } else setModeState("simple");
      })
      .catch(() => setModeState("simple"));
  }, [session]);

  return <ExperienceModeProvider mode={mode} setMode={setMode}>{children}</ExperienceModeProvider>;
}

export function useExperienceMode() {
  const value = useContext(ExperienceModeContext);
  if (!value) throw new Error("useExperienceMode must be used within ExperienceModeBoundary");
  return value;
}
