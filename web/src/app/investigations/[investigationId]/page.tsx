import { InvestigationWorkspaceController } from "@/features/investigation/InvestigationWorkspaceController";

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/u;

export default async function InvestigationPage({ params }: Readonly<{ params: Promise<{ investigationId: string }> }>) {
  const value = (await params).investigationId.toLowerCase();
  const valid = uuid.test(value);
  return <InvestigationWorkspaceController investigationId={valid ? value : ""} invalidId={!valid} />;
}
