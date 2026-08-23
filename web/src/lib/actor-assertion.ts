import { createHmac, randomUUID } from "node:crypto";

import type { Session } from "@/lib/session";

const defaultIssuer = "ledgersync-bff";
const defaultAudience = "ledgersync-private-api";
const defaultKeyId = "current";

function requiredSetting(name: string, fallback?: string): string {
  const value = process.env[name]?.trim() || fallback;
  if (!value) throw new Error(`${name} is required`);
  return value;
}

function assertionSecret(): string {
  const value = requiredSetting("LEDGERSYNC_BFF_ASSERTION_SECRET");
  if (value.length < 32) throw new Error("LEDGERSYNC_BFF_ASSERTION_SECRET must be at least 32 characters");
  return value;
}

type AssertionOptions = Readonly<{ now?: Date; assertionId?: string }>;

// NumericDate claims are Unix seconds. The session expiry remains milliseconds
// because it is a browser-cookie timestamp; conversion happens exactly once.
export function createActorAssertion(session: Session, options: AssertionOptions = {}): string {
  const now = options.now ?? new Date();
  const issuedAt = Math.floor(now.getTime() / 1000);
  const sessionExpiry = Math.floor(session.expiresAt / 1000);
  const expiresAt = Math.min(sessionExpiry, issuedAt + 60);
  if (expiresAt <= issuedAt) throw new Error("session expires before an actor assertion can be issued");

  const payload = Buffer.from(JSON.stringify({
    iss: requiredSetting("LEDGERSYNC_BFF_ASSERTION_ISSUER", defaultIssuer),
    aud: requiredSetting("LEDGERSYNC_BFF_ASSERTION_AUDIENCE", defaultAudience),
    kid: requiredSetting("LEDGERSYNC_BFF_ASSERTION_KEY_ID", defaultKeyId),
    jti: options.assertionId ?? randomUUID(),
    sub: session.subjectId,
    tenant_id: session.tenantId,
    roles: session.roles ?? [],
    scopes: session.scopes ?? [],
    iat: issuedAt,
    exp: expiresAt,
  })).toString("base64url");
  const signature = createHmac("sha256", assertionSecret()).update(Buffer.from(payload, "base64url")).digest("base64url");
  return `${payload}.${signature}`;
}
