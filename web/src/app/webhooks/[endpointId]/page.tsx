import { WebhookConsole } from "@/features/operations/WebhookConsole";
import { safeInternalReturnPath } from "@/lib/navigation";

export default async function WebhookEndpointDetailPage({ params, searchParams }: Readonly<{ params: Promise<{ endpointId: string }>; searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const { endpointId } = await params;
  const query = await searchParams;
  const returnTo = safeInternalReturnPath(query.return_to) ?? "/webhooks";
  return <WebhookConsole endpointId={endpointId} returnTo={returnTo}/>;
}
