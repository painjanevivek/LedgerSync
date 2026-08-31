import { OperationsConsole } from "@/features/operations/OperationsConsole";

function single(value: string | string[] | undefined, maximum = 256) {
  if (typeof value !== "string") return undefined;
  const clean = value.trim();
  return clean && clean.length <= maximum ? clean : undefined;
}

export default async function EventsPage({ searchParams }: Readonly<{ searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const query = await searchParams;
  return <OperationsConsole section="events" filters={{ eventType: single(query.eventType), state: single(query.state, 16), endpointId: single(query.endpointId), relatedId: single(query.relatedId), correlationId: single(query.correlationId), from: single(query.from, 64), to: single(query.to, 64), cursor: single(query.cursor, 2_048) }} />;
}
