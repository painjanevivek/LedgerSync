import { ArrowRight, WarningCircle } from "@phosphor-icons/react/dist/ssr";
import Link from "next/link";
import type { ReactNode } from "react";

import type { PresentationTone } from "@/features/console/presentation";

export function TaskCard({ title, explanation, context, action, tone = "neutral", evidence }: Readonly<{
  title: string;
  explanation: string;
  context?: ReactNode;
  action: Readonly<{ label: string; href: string }>;
  tone?: PresentationTone;
  evidence?: ReactNode;
}>) {
  return <article className="task-card" data-tone={tone}>
    <WarningCircle weight="fill" aria-hidden="true" />
    <div className="task-card-copy"><h3>{title}</h3><p>{explanation}</p>{context && <small>{context}</small>}{evidence}</div>
    <Link className="button secondary" href={action.href}>{action.label}<ArrowRight aria-hidden="true" /></Link>
  </article>;
}
