import { WebhookConsole } from "@/features/operations/WebhookConsole";

export default async function WebhookEndpointDetailPage({ params, searchParams }: Readonly<{ params: Promise<{ endpointId: string }>; searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const { endpointId } = await params;
  const query = await searchParams;
  const requested = typeof query.return_to === "string" ? query.return_to : "/webhooks";
  const returnTo = requested.startsWith("/") && !requested.startsWith("//") && requested.length <= 1_024 ? requested : "/webhooks";
  return <WebhookConsole endpointId={endpointId} returnTo={returnTo}/>;
}
