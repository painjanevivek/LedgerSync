import { ReservationsController } from "@/features/reservations/ReservationsController";
export default async function ReservationsPage({ searchParams }: Readonly<{
    searchParams: Promise<Record<string, string | string[] | undefined>>;
}>) { return <ReservationsController invalid={Object.keys(await searchParams).length > 0}/>; }
