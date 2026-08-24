import { OperationsConsole } from "@/features/operations/OperationsConsole";

export default async function EventDetailPage({ params }: Readonly<{ params: Promise<{ eventId: string }> }>) {
  const { eventId } = await params;
  return <OperationsConsole section="events" eventId={eventId} />;
}
