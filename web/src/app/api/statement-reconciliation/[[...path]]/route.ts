import { cookies } from "next/headers";
import { NextRequest } from "next/server";
import { proxyStatement } from "@/lib/statement-reconciliation-proxy";
import { readSession, sessionCookieName } from "@/lib/session";
type Context = Readonly<{
    params: Promise<{
        path?: string[];
    }>;
}>;
async function handle(request: NextRequest, context: Context) {
    const session = readSession((await cookies()).get(sessionCookieName)?.value);
    return proxyStatement(request, session, (await context.params).path ?? []);
}
export const GET = handle;
export const POST = handle;
