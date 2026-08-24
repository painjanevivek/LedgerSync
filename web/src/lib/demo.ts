import type { Session } from "@/lib/session";

export type DemoConfiguration = Readonly<{ enabled: boolean; environment: string; subjectId: string; tenantId: string }>;

const productionEnvironments = new Set(["production", "prod"]);

export function readDemoConfiguration(environment: Readonly<Record<string, string | undefined>> = process.env): DemoConfiguration {
  const enabled = environment.LEDGERSYNC_DEMO_MODE === "true";
  const deploymentEnvironment = (environment.LEDGERSYNC_DEPLOYMENT_ENV ?? "development").trim().toLowerCase();
  const hasDemoConfiguration = enabled || Boolean(environment.LEDGERSYNC_DEMO_SUBJECT_ID || environment.LEDGERSYNC_DEMO_TENANT_ID || environment.LEDGERSYNC_DEMO_DATABASE_URL);
  if (hasDemoConfiguration && productionEnvironments.has(deploymentEnvironment)) throw new Error("Demo configuration is forbidden in production");
  const subjectId = (environment.LEDGERSYNC_DEMO_SUBJECT_ID ?? "demo-operator").trim();
  const tenantId = (environment.LEDGERSYNC_DEMO_TENANT_ID ?? "00000000-0000-4000-8000-000000000001").trim();
  if (enabled && (!subjectId || !tenantId)) throw new Error("Demo subject and tenant IDs are required");
  return { enabled, environment: deploymentEnvironment, subjectId, tenantId };
}

export function createDemoSession(configuration: DemoConfiguration, now = Date.now()): Session {
  if (!configuration.enabled) throw new Error("Demo mode is disabled");
  return {
    subjectId: configuration.subjectId,
    tenantId: configuration.tenantId,
    csrfToken: crypto.randomUUID(),
    expiresAt: now + 30 * 60 * 1000,
    roles: ["tenant:operator"],
    scopes: ["accounts:read", "transactions:read", "transfers:read", "transfers:write", "reconciliation:read"],
  };
}
