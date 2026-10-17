import { cookies } from "next/headers";
import { NextRequest } from "next/server";
import { proxyReservation } from "@/lib/reservation-proxy";
import { readSession, sessionCookieName } from "@/lib/session";
type Context = Readonly<{
    params: Promise<{
        path?: string[];
    }>;
}>;
async function handle(request: NextRequest, context: Context) { return proxyReservation(request, readSession((await cookies()).get(sessionCookieName)?.value), (await context.params).path ?? []); }
export const GET = handle;
export const POST = handle;
