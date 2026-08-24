import { OperationsConsole } from "@/features/operations/OperationsConsole";

export default async function EventDetailPage({ params, searchParams }: Readonly<{ params: Promise<{ eventId: string }>; searchParams: Promise<Record<string,string|string[]|undefined>> }>) {
  const { eventId } = await params;
  const query=await searchParams; const requested=typeof query.return_to==="string"?query.return_to:"/events"; const returnTo=requested.startsWith("/")&&!requested.startsWith("//")&&requested.length<=1024?requested:"/events";
  return <OperationsConsole section="events" eventId={eventId} returnTo={returnTo} />;
}
