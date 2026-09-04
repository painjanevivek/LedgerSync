import type { ReactNode } from "react";

import { DisclosureSection } from "@/ui/disclosure/DisclosureSection";

type Props = Readonly<{
  id: string;
  children: ReactNode;
  activeCount?: number;
  invalid?: boolean;
  title?: string;
  summary?: string;
}>;

export function AdvancedFilterPanel({
  id,
  children,
  activeCount = 0,
  invalid = false,
  title = "Advanced filters",
  summary,
}: Props) {
  const stateSummary = summary ??
    (activeCount > 0
      ? `${activeCount} active filter${activeCount === 1 ? "" : "s"}`
      : "Narrow the current authorized records");
  return (
    <DisclosureSection
      id={id}
      title={title}
      summary={stateSummary}
      defaultOpen={activeCount > 0 || invalid}
      attention={invalid}
      priority="advanced"
    >
      {children}
    </DisclosureSection>
  );
}
