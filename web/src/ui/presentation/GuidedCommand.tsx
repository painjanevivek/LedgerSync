import type { ReactNode } from "react";

export function GuidedCommand({ step, total = 4, title, description, children }: Readonly<{ step: number; total?: number; title: string; description: string; children: ReactNode }>) {
  return <section className="guided-command" aria-labelledby={`guided-command-${step}`}><p className="guided-command-step">Step {step} of {total}</p><h2 id={`guided-command-${step}`}>{title}</h2><p>{description}</p>{children}</section>;
}
