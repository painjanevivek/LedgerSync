import type { ReactNode } from "react";

export function EmptyState({ title, message, action }: Readonly<{ title: string; message: string; action?: ReactNode }>) {
  return <section className="friendly-empty-state"><h2>{title}</h2><p>{message}</p>{action}</section>;
}
