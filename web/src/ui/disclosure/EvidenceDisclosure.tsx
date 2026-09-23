import type { ReactNode } from "react";

import { DisclosureSection } from "@/ui/disclosure/DisclosureSection";

export function EvidenceDisclosure({
  id,
  title,
  summary,
  children,
  attention = false,
  lazy = true,
}: Readonly<{
  id: string;
  title: string;
  summary: string;
  children: ReactNode;
  attention?: boolean;
  lazy?: boolean;
}>) {
  return (
    <DisclosureSection
      id={id}
      title={title}
      summary={summary}
      attention={attention}
      priority="advanced"
      lazy={lazy}
    >
      {children}
    </DisclosureSection>
  );
}
