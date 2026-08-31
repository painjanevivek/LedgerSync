import { ReconciliationController } from "@/features/reconciliation/ReconciliationController";

export default async function ReconciliationRunPage({ params, searchParams }: { params: Promise<{ runId: string }>; searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const { runId } = await params;
  const query=await searchParams; const requested=typeof query.return_to==="string"?query.return_to:"/reconciliation"; const returnTo=requested.startsWith("/")&&!requested.startsWith("//")&&requested.length<=1024?requested:"/reconciliation";
  return <ReconciliationController runId={runId} returnTo={returnTo} />;
}
