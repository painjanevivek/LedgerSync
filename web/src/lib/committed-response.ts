import { createSession, type Session } from "@/lib/session";

export type ConsistencyMetadataStatus = "complete" | "unavailable";

export function applyConsistencySessionMetadata(
  serialized: string,
  session: Session,
  setCookie: (signedSession: string) => void,
  signSession: (payload: Session) => string = createSession,
): ConsistencyMetadataStatus {
  try {
    if (serialized.length === 0 || serialized.length > 16_384) return "unavailable";
    const parsed: unknown = JSON.parse(serialized);
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) return "unavailable";
    const entries = Object.entries(parsed);
    if (
      entries.length > 10 ||
      entries.some(([accountId, token]) => accountId.length === 0 || accountId.length > 64 || typeof token !== "string" || token.length === 0 || token.length > 2048)
    ) {
      return "unavailable";
    }
    const consistencyRequirements = { ...session.consistencyRequirements, ...Object.fromEntries(entries) };
    if (Object.keys(consistencyRequirements).length > 10) return "unavailable";
    const signedSession = signSession({ ...session, consistencyRequirements });
    setCookie(signedSession);
    return "complete";
  } catch {
    return "unavailable";
  }
}
