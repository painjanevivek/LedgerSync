import type { Session } from "@/lib/session";
import { readPublicOrigin } from "@/lib/security";

export type LocalAccessConfiguration = Readonly<{
  enabled: boolean;
  environment: string;
  subjectId: string;
  tenantId: string;
}>;

export function isLocalSession(
  session: Session | null,
  configuration: LocalAccessConfiguration,
) {
  return Boolean(
    session &&
      configuration.enabled &&
      session.subjectId === configuration.subjectId &&
      session.tenantId === configuration.tenantId,
  );
}

/**
 * Local login is a loopback-only development adapter. It never accepts browser
 * supplied identity and production-like environments reject its configuration.
 */
export function readLocalAccessConfiguration(
  environment: Readonly<Record<string, string | undefined>> = process.env,
): LocalAccessConfiguration {
  const enabled = environment.LEDGERSYNC_LOCAL_LOGIN_ENABLED === "true";
  const deploymentEnvironment = (
    environment.LEDGERSYNC_DEPLOYMENT_ENV ?? ""
  )
    .trim()
    .toLowerCase();
  const hasLocalConfiguration =
    enabled ||
    Boolean(
      environment.LEDGERSYNC_LOCAL_SUBJECT_ID ||
        environment.LEDGERSYNC_LOCAL_TENANT_ID,
    );
  if (hasLocalConfiguration && deploymentEnvironment !== "development") {
    throw new Error(
      "Local login configuration is allowed only in explicit development mode",
    );
  }
  if (hasLocalConfiguration) {
    const origin = readPublicOrigin(environment);
    if (
      origin.hostname !== "localhost" &&
      origin.hostname !== "127.0.0.1" &&
      origin.hostname !== "[::1]"
    ) {
      throw new Error("Local login requires an explicit loopback public origin");
    }
  }
  const subjectId = (
    environment.LEDGERSYNC_LOCAL_SUBJECT_ID ?? "local-user"
  ).trim();
  const tenantId = (
    environment.LEDGERSYNC_LOCAL_TENANT_ID ??
    "00000000-0000-4000-8000-000000000001"
  ).trim();
  if (enabled && (!subjectId || !tenantId)) {
    throw new Error("Local subject and tenant IDs are required");
  }
  return {
    enabled,
    environment: deploymentEnvironment || "unconfigured",
    subjectId,
    tenantId,
  };
}

export function createLocalSession(
  configuration: LocalAccessConfiguration,
  now = Date.now(),
): Session {
  if (!configuration.enabled) throw new Error("Local login is disabled");
  return {
    subjectId: configuration.subjectId,
    tenantId: configuration.tenantId,
    csrfToken: crypto.randomUUID(),
    expiresAt: now + 30 * 60 * 1000,
    authenticatedAt: now,
    roles: ["tenant:operator"],
    scopes: [
      "accounts:read",
      "accounts:write",
      "transactions:read",
      "transfers:read",
      "transfers:write",
      "reconciliation:read",
      "reconciliation:write",
      "local:read",
      "local:write",
      "events:read",
      "investigation:read",
      "investigation:write",
      "explainability:read",
      "developer:read",
      "credentials:read",
      "credentials:write",
      "webhooks:read",
      "webhooks:write",
      "webhooks:replay",
      "recovery:read",
      "exports:read",
      "funding:read",
      "funding:write",
      "funding:approve",
      "corrections:read",
      "corrections:write",
      "corrections:approve",
    ],
  };
}

export function createLocalReturnURL(
  returnTo: string,
  environment: Readonly<Record<string, string | undefined>> = process.env,
) {
  if (!returnTo.startsWith("/") || returnTo.startsWith("//")) {
    throw new Error("Local login return path must be application-relative");
  }
  return new URL(returnTo, readPublicOrigin(environment));
}
