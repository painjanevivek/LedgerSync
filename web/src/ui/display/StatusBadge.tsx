import { CheckCircle, Info, WarningCircle, XCircle } from "@phosphor-icons/react";
import type { ReactNode } from "react";

export type StatusTone = "success" | "warning" | "danger" | "neutral" | "info";

export function StatusBadge({ children, tone = "neutral" }: Readonly<{ children: ReactNode; tone?: StatusTone }>) {
  const icon = tone === "success" ? <CheckCircle weight="fill" aria-hidden="true" /> : tone === "warning" ? <WarningCircle weight="fill" aria-hidden="true" /> : tone === "danger" ? <XCircle weight="fill" aria-hidden="true" /> : <Info weight="fill" aria-hidden="true" />;
  return <span className={`status-label ${tone}`}>{icon}{children}</span>;
}
