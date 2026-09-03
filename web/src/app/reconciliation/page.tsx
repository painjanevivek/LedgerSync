import { ReconciliationController } from "@/features/reconciliation/ReconciliationController";
import { emptyReconciliationFilters, parseReconciliationPageQuery } from "@/lib/page-query/reconciliation";

export default async function ReconciliationPage({ searchParams }: Readonly<{ searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const parsed = parseReconciliationPageQuery(await searchParams);
  return <ReconciliationController filters={parsed.ok ? parsed.filters : emptyReconciliationFilters} invalidQuery={!parsed.ok} />;
}
