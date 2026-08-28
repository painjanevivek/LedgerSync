import { cookies } from "next/headers";
import { NextRequest } from "next/server";
import {
  correctionMutationMaximumBytes,
  isCorrectionID,
  toPrivateCorrectionDecision,
} from "@/lib/api/corrections";
import {
  authorizeCorrectionMutation,
  isCorrectionDenial,
} from "@/lib/correction-boundary";
import { proxyCorrectionMutation } from "@/lib/correction-mutation";
import { jsonError, readBoundedJSON } from "@/lib/security";
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
    true,
  );
  if (isCorrectionDenial(authorization)) return authorization;
  if (!session) return jsonError("unauthorized", 401);
  const { correctionId } = await context.params;
  if (!isCorrectionID(correctionId)) return jsonError("not_found", 404);
  try {
    return proxyCorrectionMutation(
      session,
      `/api/transfer-corrections/${encodeURIComponent(correctionId)}/approve`,
      toPrivateCorrectionDecision(
        await readBoundedJSON<unknown>(request, correctionMutationMaximumBytes),
      ),
    );
  } catch {
    return jsonError("validation_failed", 400);
  }
}
