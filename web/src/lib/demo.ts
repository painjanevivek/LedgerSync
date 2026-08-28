import type { Session } from "@/lib/session";
import { readPublicOrigin } from "@/lib/security";

export type DemoConfiguration = Readonly<{ enabled: boolean; environment: string; subjectId: string; tenantId: string }>;

export function readDemoConfiguration(environment: Readonly<Record<string, string | undefined>> = process.env): DemoConfiguration {
  const enabled = environment.LEDGERSYNC_DEMO_MODE === "true";
  const deploymentEnvironment = (environment.LEDGERSYNC_DEPLOYMENT_ENV ?? "").trim().toLowerCase();
  const hasDemoConfiguration = enabled || Boolean(environment.LEDGERSYNC_DEMO_SUBJECT_ID || environment.LEDGERSYNC_DEMO_TENANT_ID || environment.LEDGERSYNC_DEMO_DATABASE_URL);
  if (hasDemoConfiguration && deploymentEnvironment !== "development") throw new Error("Demo configuration is allowed only in explicit development mode");
  if (hasDemoConfiguration) {
    const origin = readPublicOrigin(environment);
    if (origin.hostname !== "localhost" && origin.hostname !== "127.0.0.1" && origin.hostname !== "[::1]") {
      throw new Error("Demo configuration requires an explicit loopback public origin");
    }
  }
  const subjectId = (environment.LEDGERSYNC_DEMO_SUBJECT_ID ?? "demo-operator").trim();
  const tenantId = (environment.LEDGERSYNC_DEMO_TENANT_ID ?? "00000000-0000-4000-8000-000000000001").trim();
  if (enabled && (!subjectId || !tenantId)) throw new Error("Demo subject and tenant IDs are required");
  return { enabled, environment: deploymentEnvironment || "unconfigured", subjectId, tenantId };
}

export function createDemoSession(configuration: DemoConfiguration, now = Date.now()): Session {
  if (!configuration.enabled) throw new Error("Demo mode is disabled");
  return {
    subjectId: configuration.subjectId,
    tenantId: configuration.tenantId,
    csrfToken: crypto.randomUUID(),
    expiresAt: now + 30 * 60 * 1000,
    authenticatedAt: now,
    roles: ["tenant:operator"],
    scopes: ["accounts:read", "accounts:write", "transactions:read", "transfers:read", "transfers:write", "reconciliation:read", "reconciliation:write", "local:read", "local:write", "events:read", "explainability:read", "developer:read", "recovery:read", "exports:read", "funding:read", "funding:write", "funding:approve", "corrections:read", "corrections:write", "corrections:approve"],
  };
}
