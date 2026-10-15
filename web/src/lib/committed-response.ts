import { createSession, type Session } from "@/lib/session";

export type ConsistencyMetadataStatus = "complete" | "unavailable";

export function parseConsistencyRequirements(serialized: string): Readonly<Record<string, string>> | null {
  try {
    if (serialized.length === 0 || serialized.length > 16_384) return null;
    const parsed: unknown = JSON.parse(serialized);
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) return null;
    const entries = Object.entries(parsed);
    if (
      entries.length === 0 ||
      entries.length > 10 ||
      entries.some(([accountId, token]) => accountId.length === 0 || accountId.length > 64 || typeof token !== "string" || token.length === 0 || token.length > 2048)
    ) return null;
    return Object.fromEntries(entries) as Readonly<Record<string, string>>;
  } catch {
    return null;
  }
}

export function applyConsistencySessionMetadata(
  serialized: string,
  session: Session,
  setCookie: (signedSession: string) => void,
  signSession: (payload: Session) => string = createSession,
): ConsistencyMetadataStatus {
  try {
    const parsed = parseConsistencyRequirements(serialized);
    if (!parsed) return "unavailable";
    const entries = Object.entries(parsed);
    const consistencyRequirements = { ...session.consistencyRequirements, ...Object.fromEntries(entries) };
    if (Object.keys(consistencyRequirements).length > 10) return "unavailable";
    const signedSession = signSession({ ...session, consistencyRequirements });
    setCookie(signedSession);
    return "complete";
  } catch {
    return "unavailable";
  }
}
