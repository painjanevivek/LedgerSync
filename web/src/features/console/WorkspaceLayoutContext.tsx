"use client";
import { createContext, useContext, useEffect } from "react";

export type FocusedWorkspace = Readonly<{ title: string; returnTo: string; returnLabel: string }>;
export const WorkspaceLayoutContext = createContext<(value: FocusedWorkspace | null) => void>(() => undefined);
export function useFocusedWorkspace(title: string, returnTo: string, returnLabel: string) {
  const setLayout = useContext(WorkspaceLayoutContext);
  useEffect(() => { setLayout({ title, returnTo, returnLabel }); return () => setLayout(null); }, [returnLabel, returnTo, setLayout, title]);
}
