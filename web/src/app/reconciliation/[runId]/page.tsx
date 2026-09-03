import { ReconciliationController } from "@/features/reconciliation/ReconciliationController";
import { safeInternalReturnPath } from "@/lib/navigation";

export default async function ReconciliationRunPage({ params, searchParams }: { params: Promise<{ runId: string }>; searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const { runId } = await params;
  const query = await searchParams;
  const returnTo = safeInternalReturnPath(query.return_to) ?? "/reconciliation";
  return <ReconciliationController runId={runId} returnTo={returnTo} />;
}
