"use client";
import { CsvEvidenceDownload } from "@/ui/controls/CsvEvidenceDownload.client";
export function StatementDownload({ path, id, label, source = false, disabled = false }: Readonly<{
  path: string; id: string; label: string; source?: boolean; disabled?: boolean;
}>) {
  return <CsvEvidenceDownload path={path} filename={`statement-${id}.csv`} label={label} maxBytes={source ? 1048576 : 4194304} digestHeader={source ? "x-statement-sha256" : undefined} disabled={disabled}/>;
}
