import type { ReactNode } from "react";
import type { PresentationStatus } from "@/features/console/presentation";

export function StatusSummary({ status, action }: Readonly<{ status: PresentationStatus; action?: ReactNode }>) {
  return <section className="status-summary" data-tone={status.tone}><div><h2>{status.title}</h2><p>{status.explanation}</p></div>{action}</section>;
}
