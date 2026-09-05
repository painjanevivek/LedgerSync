import type { ReactNode } from "react";

export function TechnicalDetails({ summary = "View details", attention = false, children }: Readonly<{
  summary?: string;
  attention?: boolean;
  children: ReactNode;
}>) {
  return (
    <details className="technical-details" open={attention || undefined} data-attention={attention || undefined}>
      <summary>{summary}</summary>
      <div className="technical-details-body">{children}</div>
    </details>
  );
}
