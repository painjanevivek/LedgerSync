import { createHmac, timingSafeEqual } from "node:crypto";

export const sessionCookieName = "ledgersync_session";

export type Session = Readonly<{
  subjectId: string;
  tenantId: string;
  csrfToken: string;
  expiresAt: number;
  roles?: readonly string[];
  scopes?: readonly string[];
  consistencyRequirements?: Readonly<Record<string, string>>;
}>;

type SignedSession = Session & { signature: string };

function secret(): string {
  const value = process.env.LEDGERSYNC_WEB_SESSION_SECRET;
  if (!value || value.length < 32) {
    throw new Error("LEDGERSYNC_WEB_SESSION_SECRET must be at least 32 characters");
  }
  return value;
}

function sign(payload: Session): string {
  return createHmac("sha256", secret()).update(JSON.stringify(payload)).digest("base64url");
}

export function createSession(payload: Session): string {
  const signed: SignedSession = { ...payload, signature: sign(payload) };
  return Buffer.from(JSON.stringify(signed)).toString("base64url");
}

export function readSession(raw: string | undefined): Session | null {
  if (!raw) return null;
  try {
    const parsed = JSON.parse(Buffer.from(raw, "base64url").toString("utf8")) as Partial<SignedSession>;
    if (
      typeof parsed.subjectId !== "string" ||
      typeof parsed.tenantId !== "string" ||
      typeof parsed.csrfToken !== "string" ||
      typeof parsed.expiresAt !== "number" ||
      typeof parsed.signature !== "string" ||
      parsed.expiresAt <= Date.now()
    ) {
      return null;
    }
    const requirements = parsed.consistencyRequirements;
    if (requirements !== undefined && (typeof requirements !== "object" || requirements === null || Object.entries(requirements).length > 10 || Object.entries(requirements).some(([accountId, token]) => typeof accountId !== "string" || typeof token !== "string" || token.length > 2048))) return null;
    const payload: Session = {
      subjectId: parsed.subjectId,
      tenantId: parsed.tenantId,
      csrfToken: parsed.csrfToken,
      expiresAt: parsed.expiresAt,
      roles: validStringList(parsed.roles) ? parsed.roles : undefined,
      scopes: validStringList(parsed.scopes) ? parsed.scopes : undefined,
      consistencyRequirements: requirements as Readonly<Record<string, string>> | undefined,
    };
    const expected = Buffer.from(sign(payload));
    const supplied = Buffer.from(parsed.signature);
    return expected.length === supplied.length && timingSafeEqual(expected, supplied) ? payload : null;
  } catch {
    return null;
  }
}

function validStringList(value: unknown): value is string[] {
  return Array.isArray(value) && value.length <= 16 && value.every((item) => typeof item === "string" && item.length > 0 && item.length <= 64);
}

export function sessionCookie(value: string) {
  return {
    name: sessionCookieName,
    value,
    httpOnly: true,
    sameSite: "lax" as const,
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: 60 * 30,
  };
}
