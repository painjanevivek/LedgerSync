import { createHmac, timingSafeEqual } from "node:crypto";

import { readPublicOrigin } from "@/lib/security";
import { getPrivateAPIWorkloadCredential } from "@/lib/workload-credential";
import { emitGuardrailMetric } from "@/lib/guardrail-metrics";
import { authenticationCookiePolicy } from "@/lib/cookie-policy";

export const sessionCookieName = authenticationCookiePolicy().sessionName;
export const maxSessionCookieValueBytes = 3_584;
export const opaqueSessionTokenBytes = 32;

const maxSessionRoles = 16;
const maxSessionScopes = 32;
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
  const policy = authenticationCookiePolicy();
  return {
    name: policy.sessionName,
    value,
    httpOnly: true,
    sameSite: "lax" as const,
    secure: policy.secure,
    path: "/",
    maxAge: 60 * 30,
  };
}

export function expiredSessionCookie() {
  return { ...sessionCookie(""), maxAge: 0, expires: new Date(0) };
}

type SessionStoreResponse = Readonly<{
  token?: unknown;
  subject_id?: unknown;
  tenant_id?: unknown;
  csrf_token?: unknown;
  expires_at?: unknown;
  authenticated_at?: unknown;
  roles?: unknown;
  scopes?: unknown;
  consistency_requirements?: unknown;
}>;

async function sessionStoreRequest(
  path: string,
  method: "POST" | "PATCH",
  body: Readonly<Record<string, unknown>>,
): Promise<SessionStoreResponse | null> {
  const apiURL = process.env.LEDGERSYNC_PRIVATE_API_URL?.trim().replace(/\/$/, "");
  if (!apiURL) throw new Error("private session store is unavailable");
  let response: Response;
  try {
    response = await fetch(`${apiURL}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${await getPrivateAPIWorkloadCredential()}`,
      "Content-Type": "application/json",
      Accept: "application/json",
      "X-Request-ID": crypto.randomUUID(),
    },
    body: JSON.stringify(body),
    cache: "no-store",
    signal: AbortSignal.timeout(3_000),
    });
  } catch {
    emitGuardrailMetric("session", "unavailable");
    throw new Error("private session store request failed");
  }
  if (response.status === 401 || response.status === 404) {
    emitGuardrailMetric("session", "rejected");
    return null;
  }
  if (!response.ok) {
    emitGuardrailMetric("session", "unavailable");
    throw new Error("private session store request failed");
  }
  const outcome = path.endsWith("/resolve")
    ? "resolved"
    : path.endsWith("/revoke")
      ? "revoked"
      : path.endsWith("/consistency")
        ? "consistency_updated"
        : body.rotate_token
          ? "rotated"
          : "created";
  emitGuardrailMetric("session", outcome);
  if (response.status === 204) return {};
  const raw = await response.text();
  if (Buffer.byteLength(raw, "utf8") > 16 * 1024) throw new Error("private session response is too large");
  try {
    const value = JSON.parse(raw) as unknown;
    return value && typeof value === "object" && !Array.isArray(value) ? value as SessionStoreResponse : null;
  } catch {
    throw new Error("private session response is malformed");
  }
}

function opaqueToken(value: unknown): value is string {
  return typeof value === "string" && /^[A-Za-z0-9_-]{43}$/.test(value);
}

function sessionFromStore(value: SessionStoreResponse | null): Session | null {
  if (!value || typeof value.subject_id !== "string" || typeof value.tenant_id !== "string" || typeof value.csrf_token !== "string" || typeof value.expires_at !== "number" || value.expires_at <= Date.now()) return null;
  if (!validStringList(value.roles, maxSessionRoles) || !validStringList(value.scopes, maxSessionScopes)) return null;
  const requirements = value.consistency_requirements;
  if (requirements !== undefined && (typeof requirements !== "object" || requirements === null || Array.isArray(requirements) || Object.entries(requirements).length > maxConsistencyRequirements || Object.entries(requirements).some(([accountId, token]) => accountId.length < 1 || accountId.length > maxConsistencyAccountIDLength || typeof token !== "string" || token.length < 1 || token.length > maxConsistencyTokenLength))) return null;
  return {
    subjectId: value.subject_id,
    tenantId: value.tenant_id,
    csrfToken: value.csrf_token,
    expiresAt: value.expires_at,
    roles: value.roles,
    scopes: value.scopes,
    consistencyRequirements: requirements as Record<string, string> | undefined,
    ...(typeof value.authenticated_at === "number" && value.authenticated_at > 0 ? { authenticatedAt: value.authenticated_at } : {}),
  };
}

export async function createOpaqueSession(payload: Session, rotateToken?: string): Promise<string> {
  const bounded = boundedSession(payload);
  const response = await sessionStoreRequest("/api/internal/bff/sessions", "POST", {
    subject_id: bounded.subjectId,
    tenant_id: bounded.tenantId,
    csrf_token: bounded.csrfToken,
    expires_at: bounded.expiresAt,
    authenticated_at: bounded.authenticatedAt ?? 0,
    roles: bounded.roles ?? [],
    scopes: bounded.scopes ?? [],
    ...(rotateToken && opaqueToken(rotateToken) ? { rotate_token: rotateToken } : {}),
  });
  if (!opaqueToken(response?.token)) throw new Error("private session creation failed");
  return response.token;
}

export async function resolveOpaqueSession(token: string): Promise<Session | null> {
  if (!opaqueToken(token)) return null;
  return sessionFromStore(await sessionStoreRequest("/api/internal/bff/sessions/resolve", "POST", { token }));
}

export async function revokeOpaqueSession(token: string): Promise<boolean> {
  if (!opaqueToken(token)) return false;
  return (await sessionStoreRequest("/api/internal/bff/sessions/revoke", "POST", { token })) !== null;
}

export async function updateOpaqueSessionConsistency(token: string, requirements: Readonly<Record<string, string>>): Promise<boolean> {
  if (!opaqueToken(token)) return false;
  return (await sessionStoreRequest("/api/internal/bff/sessions/consistency", "PATCH", { token, consistency_requirements: requirements })) !== null;
}
