import { createHmac } from "node:crypto";

import type { Session } from "@/lib/session";

function assertionSecret(): string {
  const value = process.env.LEDGERSYNC_BFF_ASSERTION_SECRET;
  if (!value || value.length < 32) throw new Error("LEDGERSYNC_BFF_ASSERTION_SECRET must be at least 32 characters");
  return value;
}

// The API only accepts this after separately authenticating the BFF workload
// with bff:act-as-user. It is short-lived and never reaches browser JavaScript.
export function createActorAssertion(session: Session): string {
  const payload = Buffer.from(JSON.stringify({ sub: session.subjectId, tenant_id: session.tenantId, roles: session.roles ?? [], scopes: session.scopes ?? [], exp: Math.min(session.expiresAt, Date.now() + 60_000) })).toString("base64url");
  const signature = createHmac("sha256", assertionSecret()).update(Buffer.from(payload, "base64url")).digest("base64url");
  return `${payload}.${signature}`;
}
