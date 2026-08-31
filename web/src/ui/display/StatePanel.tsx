import { Info, WarningCircle } from "@phosphor-icons/react";
import type { ReactNode } from "react";

export type StatePanelKind = "empty" | "error" | "offline" | "denied" | "unknown";
export type StatePanelAnnouncement = "polite" | "assertive";

export function StatePanel({ title, message, kind = "empty", action, announce }: Readonly<{
  title: string;
  message: string;
  kind?: StatePanelKind;
  action?: ReactNode;
  announce?: StatePanelAnnouncement;
}>) {
  const role = kind === "error" || announce === "assertive" ? "alert" : announce === "polite" ? "status" : undefined;
  const live = role ? (role === "alert" ? "assertive" : "polite") : undefined;
  return <div className={`state-panel ${kind}`} role={role} aria-live={live}>{kind === "error" || kind === "offline" || kind === "unknown" ? <WarningCircle weight="fill" aria-hidden="true" /> : <Info weight="fill" aria-hidden="true" />}<div><strong role="heading" aria-level={3}>{title}</strong><p>{message}</p>{action}</div></div>;
}
