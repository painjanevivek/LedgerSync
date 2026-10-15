"use client";

import { usePathname } from "next/navigation";
import { createContext, useContext, useState, type ReactNode } from "react";
import type { ConsoleSession } from "@/features/accounts/types";
import { ConsoleSessionBoundary } from "./ConsoleSessionBoundary";
import { ExperienceModeBoundary } from "./ExperienceModeBoundary";

export function WorkspaceProviders({ children, initialSession, onResolved }: Readonly<{ children: ReactNode; initialSession?: ConsoleSession; onResolved?: (session: ConsoleSession) => void }>) {
  return <ConsoleSessionBoundary initialSession={initialSession} onResolved={onResolved}><ExperienceModeBoundary>{children}</ExperienceModeBoundary></ConsoleSessionBoundary>;
}

/** Public entrypoints must not mount preference or financial data providers. */
export function RouteProviders({ children }: Readonly<{ children: ReactNode }>) {
  const pathname = usePathname();
  if (["/welcome", "/sign-in"].includes(pathname)) return children;
  return <WorkspaceGate>{children}</WorkspaceGate>;
}

const RootEntryContext = createContext<{ mounted: boolean; acceptSession: (session: ConsoleSession) => void }>({ mounted: false, acceptSession: () => undefined });
export const useRootEntry = () => useContext(RootEntryContext);

/** Keep the proven session boundary mounted when Home navigates into the workspace. */
function WorkspaceGate({ children }: Readonly<{ children: ReactNode }>) {
  const pathname = usePathname();
  const [initialSession, acceptSession] = useState<ConsoleSession>();
  const mounted = pathname !== "/" || Boolean(initialSession);
  return <RootEntryContext.Provider value={{ mounted, acceptSession }}>
    {mounted ? <WorkspaceProviders initialSession={initialSession} onResolved={acceptSession}>{children}</WorkspaceProviders> : children}
  </RootEntryContext.Provider>;
}
