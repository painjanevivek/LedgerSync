import { createHmac, timingSafeEqual } from "node:crypto";

import { readPublicOrigin } from "@/lib/security";

export const sessionCookieName = "ledgersync_session";

const maxSessionRoles = 16;
const maxSessionScopes = 32;

export type Session = Readonly<{
  subjectId: string;
  tenantId: string;
  csrfToken: string;
  expiresAt: number;
  roles?: readonly string[];
  scopes?: readonly string[];
  consistencyRequirements?: Readonly<Record<string, string>>;
}>;

function secret(): string {
  const value = process.env.LEDGERSYNC_WEB_SESSION_SECRET;
  if (!value || value.length < 32) {
    throw new Error("LEDGERSYNC_WEB_SESSION_SECRET must be at least 32 characters");
  }
  return value;
}

function sign(encodedPayload: string): string {
  return createHmac("sha256", secret()).update(encodedPayload).digest("base64url");
}

export function createSession(payload: Session): string {
  const encodedPayload = Buffer.from(JSON.stringify(payload)).toString("base64url");
  return `${encodedPayload}.${sign(encodedPayload)}`;
}

export function readSession(raw: string | undefined): Session | null {
  if (!raw) return null;
  try {
    const parts = raw.split(".");
    if (parts.length !== 2 || parts[0].length > 16_384 || parts[1].length > 256) return null;
    const expected = Buffer.from(sign(parts[0]));
    const supplied = Buffer.from(parts[1]);
    if (expected.length !== supplied.length || !timingSafeEqual(expected, supplied)) return null;
    const parsed = JSON.parse(Buffer.from(parts[0], "base64url").toString("utf8")) as Partial<Session>;
    if (
      typeof parsed.subjectId !== "string" ||
      typeof parsed.tenantId !== "string" ||
      typeof parsed.csrfToken !== "string" ||
      typeof parsed.expiresAt !== "number" ||
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
      roles: validStringList(parsed.roles, maxSessionRoles) ? parsed.roles : undefined,
      scopes: validStringList(parsed.scopes, maxSessionScopes) ? parsed.scopes : undefined,
      consistencyRequirements: requirements as Readonly<Record<string, string>> | undefined,
    };
    return payload;
  } catch {
    return null;
  }
}

function validStringList(value: unknown, maximumItems: number): value is string[] {
  return Array.isArray(value) && value.length <= maximumItems && value.every((item) => typeof item === "string" && item.length > 0 && item.length <= 64);
}

export function sessionCookie(value: string) {
  readPublicOrigin();
  const deploymentEnvironment = (process.env.LEDGERSYNC_DEPLOYMENT_ENV ?? process.env.NODE_ENV ?? "development").trim().toLowerCase();
  const production = deploymentEnvironment === "production" || deploymentEnvironment === "prod";
  const explicitlyInsecureLocal = process.env.LEDGERSYNC_COOKIE_SECURE === "false" && !production;
  return {
    name: sessionCookieName,
    value,
    httpOnly: true,
    sameSite: "lax" as const,
    secure: !explicitlyInsecureLocal,
    path: "/",
    maxAge: 60 * 30,
  };
}
