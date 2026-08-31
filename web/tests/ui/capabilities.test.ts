import assert from "node:assert/strict";
import test from "node:test";

import type { ConsoleSession } from "../../src/features/accounts/types";
import {
  canOpenApprovalInbox,
  canOpenEventsAndWebhooks,
  canSearchInvestigations,
  canOpenOrientationStep,
  deriveConsoleCapabilities,
} from "../../src/features/console/capabilities";

function session(
  environment: "local" | "production",
  scopes: string[],
): ConsoleSession {
  return {
    subject_id: "operator-1",
    tenant_id: "tenant-1",
    csrf_token: "csrf",
    environment,
    scopes,
  };
}

test("local-only capabilities require both the local environment and server scope", () => {
  const local = deriveConsoleCapabilities(
    session("local", ["local:read", "local:write"]),
  );
  const production = deriveConsoleCapabilities(
    session("production", ["local:read", "local:write"]),
  );

  assert.equal(local.localDiagnosticsRead, true);
  assert.equal(local.localOrientationWrite, true);
  assert.equal(production.localDiagnosticsRead, false);
  assert.equal(production.localOrientationWrite, false);
});

test("approval and webhook navigation follows server-issued capability families", () => {
  const fundingApprover = deriveConsoleCapabilities(
    session("production", ["funding:read", "funding:approve"]),
  );
  const webhookOperator = deriveConsoleCapabilities(
    session("production", ["webhooks:read", "webhooks:replay"]),
  );

  assert.equal(canOpenApprovalInbox(fundingApprover), true);
  assert.equal(canOpenEventsAndWebhooks(fundingApprover), false);
  assert.equal(canOpenEventsAndWebhooks(webhookOperator), true);
  assert.equal(webhookOperator.webhooksManage, true);
});

test("administration stays unreleased even when a browser session invents a scope", () => {
  const capabilities = deriveConsoleCapabilities(
    session("production", ["administration:manage"]),
  );
  assert.equal(capabilities.administrationManage, false);
});

test("cross-domain search requires its dedicated entry scope and a readable domain", () => {
  const entryOnly = deriveConsoleCapabilities(session("production", ["investigation:read"]));
  const domainOnly = deriveConsoleCapabilities(session("production", ["accounts:read"]));
  const authorized = deriveConsoleCapabilities(session("production", ["investigation:read", "accounts:read"]));
  assert.equal(canSearchInvestigations(entryOnly), false);
  assert.equal(canSearchInvestigations(domainOnly), false);
  assert.equal(canSearchInvestigations(authorized), true);
});

test("recommended setup steps are eligible only when their capability exists", () => {
  const reader = deriveConsoleCapabilities(
    session("local", ["accounts:read", "transfers:read", "local:read"]),
  );

  assert.equal(canOpenOrientationStep(reader, "confirm_health"), true);
  assert.equal(canOpenOrientationStep(reader, "inspect_accounts"), true);
  assert.equal(canOpenOrientationStep(reader, "create_account"), false);
  assert.equal(canOpenOrientationStep(reader, "post_transfer"), false);
  assert.equal(canOpenOrientationStep(reader, "inspect_postings"), true);
});
