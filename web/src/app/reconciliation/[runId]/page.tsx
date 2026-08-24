import { OperatorConsole } from "@/features/accounts/OperatorConsole";

export default async function ReconciliationRunPage({ params }: { params: Promise<{ runId: string }> }) {
  const { runId } = await params;
  return <OperatorConsole initialSection="reconciliation" initialReconciliationRunId={runId} />;
}
