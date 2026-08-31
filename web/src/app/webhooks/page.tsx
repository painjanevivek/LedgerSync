import { WebhookConsole } from "@/features/operations/WebhookConsole";

function single(value: string | string[] | undefined, maximum = 256) {
  if (typeof value !== "string") return undefined;
  const clean = value.trim();
  return clean && clean.length <= maximum ? clean : undefined;
}

export default async function WebhookEndpointsPage({ searchParams }: Readonly<{ searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const query = await searchParams;
  return <WebhookConsole filters={{ status: single(query.status, 32), eventType: single(query.eventType, 128), cursor: single(query.cursor, 2_048) }}/>;
}
