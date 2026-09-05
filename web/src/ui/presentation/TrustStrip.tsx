import { CheckCircle, Info, WarningCircle, WifiSlash } from "@phosphor-icons/react/dist/ssr";
import type { ReactNode } from "react";

import type { PresentationTone } from "@/features/console/presentation";

export function TrustStrip({ tone, title, detail, action }: Readonly<{
  tone: PresentationTone;
  title: string;
  detail?: ReactNode;
  action?: ReactNode;
}>) {
  const Icon = tone === "positive" ? CheckCircle : tone === "danger" || tone === "warning" ? WarningCircle : tone === "unknown" ? WifiSlash : Info;
  return (
    <section className="trust-strip" data-tone={tone} aria-label="Balance confidence">
      <Icon weight="fill" aria-hidden="true" />
      <div><strong>{title}</strong>{detail && <span>{detail}</span>}</div>
      {action && <div className="trust-strip-action">{action}</div>}
    </section>
  );
}
