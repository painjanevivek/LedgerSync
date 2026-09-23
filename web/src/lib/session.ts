import { createHmac, timingSafeEqual } from "node:crypto";

import { readPublicOrigin } from "@/lib/security";

export const sessionCookieName = "ledgersync_session";
export const maxSessionCookieValueBytes = 3_584;

const maxSessionRoles = 16;
const maxSessionScopes = 40;
const maxConsistencyRequirements = 10;
const maxConsistencyAccountIDLength = 128;
const maxConsistencyTokenLength = 2_048;

export type Session = Readonly<{
  subjectId: string;
  tenantId: string;
  csrfToken: string;
  expiresAt: number;
  authenticatedAt?: number;
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

function encodeSession(payload: Session): string {
  const encodedPayload = Buffer.from(JSON.stringify(payload)).toString("base64url");
  return `${encodedPayload}.${sign(encodedPayload)}`;
}

function boundedSession(payload: Session): Session {
  const base: Session = { ...payload, consistencyRequirements: undefined };
  if (encodeSession(base).length > maxSessionCookieValueBytes) {
    throw new Error("session identity and authorization claims exceed the cookie budget");
  }
  const entries = Object.entries(payload.consistencyRequirements ?? {})
    .filter(([accountId, token]) => accountId.length > 0 && accountId.length <= maxConsistencyAccountIDLength && token.length <= maxConsistencyTokenLength)
    .slice(-maxConsistencyRequirements);
  const retained: Array<readonly [string, string]> = [];
  // Transfers append new requirements after existing ones. Work newest-first so
  // read-your-writes evidence for the latest command wins when space is tight.
  for (let index = entries.length - 1; index >= 0; index -= 1) {
    const candidate = [entries[index], ...retained];
    if (encodeSession({ ...base, consistencyRequirements: Object.fromEntries(candidate) }).length <= maxSessionCookieValueBytes) {
      retained.unshift(entries[index]);
    }
  }
  return {
    ...base,
    consistencyRequirements: retained.length > 0 ? Object.fromEntries(retained) : undefined,
  };
}

export function createSession(payload: Session): string {
  return encodeSession(boundedSession(payload));
}

export function readSession(raw: string | undefined): Session | null {
  if (!raw) return null;
  try {
    const parts = raw.split(".");
    if (raw.length > maxSessionCookieValueBytes || parts.length !== 2 || parts[1].length > 64) return null;
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
    if (requirements !== undefined && (typeof requirements !== "object" || requirements === null || Object.entries(requirements).length > maxConsistencyRequirements || Object.entries(requirements).some(([accountId, token]) => accountId.length === 0 || accountId.length > maxConsistencyAccountIDLength || typeof token !== "string" || token.length > maxConsistencyTokenLength))) return null;
    const payload: Session = {
      subjectId: parsed.subjectId,
      tenantId: parsed.tenantId,
      csrfToken: parsed.csrfToken,
      expiresAt: parsed.expiresAt,
      roles: validStringList(parsed.roles, maxSessionRoles) ? parsed.roles : undefined,
      scopes: validStringList(parsed.scopes, maxSessionScopes) ? parsed.scopes : undefined,
      consistencyRequirements: requirements as Readonly<Record<string, string>> | undefined,
      ...(typeof parsed.authenticatedAt === "number" && parsed.authenticatedAt > 0 && parsed.authenticatedAt <= Date.now() + 30_000 ? { authenticatedAt: parsed.authenticatedAt } : {}),
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
