import type { ReactNode } from "react";

export function DataTableRegion({ label, children, caption, resultSummary, sortDescription }: Readonly<{
  label: string;
  children: ReactNode;
  caption?: ReactNode;
  resultSummary?: ReactNode;
  sortDescription?: ReactNode;
}>) {
  return <div className="data-table-wrap" role="region" aria-label={label} tabIndex={0}>
    {caption}
    {resultSummary}
    {sortDescription}
    <p className="table-scroll-hint">Scroll horizontally to inspect every exact field.</p>
    {children}
  </div>;
}
