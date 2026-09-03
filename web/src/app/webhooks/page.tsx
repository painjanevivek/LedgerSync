import { WebhookConsole } from "@/features/operations/WebhookConsole";
import { emptyWebhookFilters, parseWebhookPageQuery } from "@/lib/page-query/operations";

export default async function WebhookEndpointsPage({ searchParams }: Readonly<{ searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const query = await searchParams;
  const parsed = parseWebhookPageQuery(query);
  return <WebhookConsole filters={parsed.ok ? parsed.filters : emptyWebhookFilters} invalidQuery={!parsed.ok}/>;
}
