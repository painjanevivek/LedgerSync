import { cookies } from "next/headers";
import { NextRequest } from "next/server";

import {
  correctionMutationMaximumBytes,
  isCorrectionID,
  toPrivateCorrectionRequest,
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
  context: { params: Promise<{ transferId: string }> },
) {
  const session = readSession((await cookies()).get(sessionCookieName)?.value);
  const authorization = authorizeCorrectionMutation(
    request,
    session,
    "corrections:write",
    true,
    true,
  );
  if (isCorrectionDenial(authorization)) return authorization;
  if (!session || !authorization.idempotencyKey)
    return jsonError("unauthorized", 401);
  const { transferId } = await context.params;
  if (!isCorrectionID(transferId)) return jsonError("not_found", 404);
  try {
    const body = toPrivateCorrectionRequest(
      await readBoundedJSON<unknown>(request, correctionMutationMaximumBytes),
    );
    return proxyCorrectionMutation(
      session,
      `/api/transfers/${encodeURIComponent(transferId)}/corrections`,
      body,
      authorization.idempotencyKey,
    );
  } catch {
    return jsonError("validation_failed", 400);
  }
}
