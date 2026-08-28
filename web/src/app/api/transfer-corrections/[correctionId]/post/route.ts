import { cookies } from "next/headers";
import { NextRequest } from "next/server";
import { isCorrectionID } from "@/lib/api/corrections";
import {
  authorizeCorrectionMutation,
  isCorrectionDenial,
} from "@/lib/correction-boundary";
import { proxyCorrectionMutation } from "@/lib/correction-mutation";
import { jsonError } from "@/lib/security";
import { readSession, sessionCookieName } from "@/lib/session";

export async function POST(
  request: NextRequest,
  context: { params: Promise<{ correctionId: string }> },
) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = authorizeCorrectionMutation(
    request,
    session,
    "corrections:approve",
    false,
    false,
  );
  if (isCorrectionDenial(authorization)) return authorization;
  if (!session) return jsonError("unauthorized", 401);
  const { correctionId } = await context.params;
  if (!isCorrectionID(correctionId)) return jsonError("not_found", 404);
  return proxyCorrectionMutation(
    session,
    `/api/transfer-corrections/${encodeURIComponent(correctionId)}/post`,
  );
}
