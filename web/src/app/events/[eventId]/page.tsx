import { OperationsConsole } from "@/features/operations/OperationsConsole";
import { safeInternalReturnPath } from "@/lib/navigation";

export default async function EventDetailPage({ params, searchParams }: Readonly<{ params: Promise<{ eventId: string }>; searchParams: Promise<Record<string,string|string[]|undefined>> }>) {
  const { eventId } = await params;
  const query = await searchParams;
  const returnTo = safeInternalReturnPath(query.return_to) ?? "/events";
  return <OperationsConsole section="events" eventId={eventId} returnTo={returnTo} />;
}
