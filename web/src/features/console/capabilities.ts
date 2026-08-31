import type { ConsoleSession } from "@/features/accounts/types";
import type { OrientationStep } from "@/lib/api/orientation";

export type ConsoleCapabilities = Readonly<{
  accountsRead: boolean;
  accountsWrite: boolean;
  transfersRead: boolean;
  transfersWrite: boolean;
  fundingRead: boolean;
  fundingWrite: boolean;
  fundingApprove: boolean;
  correctionsRead: boolean;
  correctionsWrite: boolean;
  correctionsApprove: boolean;
  reconciliationRead: boolean;
  reconciliationWrite: boolean;
  eventsRead: boolean;
  investigationRead: boolean;
  investigationWrite: boolean;
  webhooksRead: boolean;
  webhooksManage: boolean;
  recoveryRead: boolean;
  developerRead: boolean;
  localDiagnosticsRead: boolean;
  localOrientationWrite: boolean;
  administrationManage: false;
}>;

const noCapabilities: ConsoleCapabilities = {
  accountsRead: false,
  accountsWrite: false,
  transfersRead: false,
  transfersWrite: false,
  fundingRead: false,
  fundingWrite: false,
  fundingApprove: false,
  correctionsRead: false,
  correctionsWrite: false,
  correctionsApprove: false,
  reconciliationRead: false,
  reconciliationWrite: false,
  eventsRead: false,
  investigationRead: false,
  investigationWrite: false,
  webhooksRead: false,
  webhooksManage: false,
  recoveryRead: false,
  developerRead: false,
  localDiagnosticsRead: false,
  localOrientationWrite: false,
  administrationManage: false,
};

/**
 * Converts the browser-safe, server-issued session into presentation
 * capabilities. This controls discoverability only; BFF and private API
 * authorization remain authoritative for every direct request.
 */
export function deriveConsoleCapabilities(
  session: ConsoleSession | null,
): ConsoleCapabilities {
  if (!session) return noCapabilities;
  const scopes = new Set(session.scopes);
  const local = session.environment === "local";
  return {
    accountsRead: scopes.has("accounts:read"),
    accountsWrite: scopes.has("accounts:write"),
    transfersRead: scopes.has("transfers:read"),
    transfersWrite: scopes.has("transfers:write"),
    fundingRead: scopes.has("funding:read"),
    fundingWrite: scopes.has("funding:write"),
    fundingApprove: scopes.has("funding:approve"),
    correctionsRead: scopes.has("corrections:read"),
    correctionsWrite: scopes.has("corrections:write"),
    correctionsApprove: scopes.has("corrections:approve"),
    reconciliationRead: scopes.has("reconciliation:read"),
    reconciliationWrite: scopes.has("reconciliation:write"),
    eventsRead: scopes.has("events:read"),
    investigationRead: scopes.has("investigation:read"),
    investigationWrite: scopes.has("investigation:write"),
    webhooksRead: scopes.has("webhooks:read"),
    webhooksManage:
      scopes.has("webhooks:write") || scopes.has("webhooks:replay"),
    recoveryRead: scopes.has("recovery:read"),
    developerRead: scopes.has("developer:read"),
    localDiagnosticsRead: local && scopes.has("local:read"),
    localOrientationWrite: local && scopes.has("local:write"),
    // Production administration is intentionally unreleased. It cannot become
    // visible through an invented browser scope before the server contract,
    // privileged route boundary, and Security approval exist.
    administrationManage: false,
  };
}

export function canOpenApprovalInbox(capabilities: ConsoleCapabilities) {
  return capabilities.fundingApprove || capabilities.correctionsApprove;
}

export function canOpenEventsAndWebhooks(capabilities: ConsoleCapabilities) {
  return (
    capabilities.eventsRead ||
    capabilities.webhooksRead ||
    capabilities.webhooksManage
  );
}

export function canSearchInvestigations(capabilities: ConsoleCapabilities) {
  return capabilities.investigationRead && (capabilities.accountsRead || capabilities.transfersRead || capabilities.fundingRead || capabilities.eventsRead || capabilities.reconciliationRead || capabilities.correctionsRead);
}

export function canOpenOrientationStep(
  capabilities: ConsoleCapabilities,
  step: OrientationStep["id"],
) {
  switch (step) {
    case "confirm_health":
      return capabilities.localDiagnosticsRead;
    case "understand_authority":
      return true;
    case "inspect_accounts":
      return capabilities.accountsRead;
    case "create_account":
      return capabilities.accountsWrite;
    case "fund_account":
      return capabilities.fundingRead;
    case "post_transfer":
    case "retry_transfer":
      return capabilities.transfersWrite;
    case "inspect_postings":
      return capabilities.transfersRead;
    case "run_reconciliation":
      return capabilities.reconciliationRead;
    case "inspect_delivery":
      return canOpenEventsAndWebhooks(capabilities);
    case "export_evidence":
      return capabilities.transfersRead;
    case "create_backup":
      return capabilities.recoveryRead;
  }
}
