import { execFileSync } from "node:child_process";

import { expect, type APIResponse, type Page, type Request } from "@playwright/test";

import { parseIsolatedComposeProject, parseSystemWebURL } from "./real-stack-boundary";

export type RealStackRun = {
  baseURL: string;
  composeProject: string;
  postgresContainerID: string;
  sourceAccountID: string;
  runID: string;
};

export type DurableEvidenceExpectation = Readonly<{
  accountID: string;
  externalReference: string;
  createIdempotencyKey: string;
  deniedCloseIdempotencyKey: string;
  fundingTransferID: string;
  fundingIdempotencyKey: string;
  returnTransferID: string;
  returnIdempotencyKey: string;
  reconciliationRunID: string;
  reconciliationIdempotencyKey: string;
}>;

export type DurableEvidence = Readonly<Record<
  | "account_count"
  | "owner_count"
  | "zero_closed_projection_count"
  | "create_idempotency_count"
  | "denied_close_idempotency_count"
  | "successful_account_audit_count"
  | "denied_close_audit_count"
  | "account_outbox_count"
  | "transfer_count"
  | "transfer_idempotency_count"
  | "transfer_audit_count"
  | "transfer_outbox_count"
  | "posting_count"
  | "balanced_transfer_count"
  | "reconciliation_count"
  | "reconciliation_idempotency_count"
  | "reconciliation_request_audit_count"
  | "reconciliation_completed_audit_count"
  | "active_reconciliation_command_count",
  number
>>;

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

function dockerOutput(args: string[], input?: string): string {
  try {
    return execFileSync("docker", args, { encoding: "utf8", input, timeout: 15_000, windowsHide: true }).trim();
  } catch {
    throw new Error("the isolated Docker Compose boundary could not be verified");
  }
}

function requireIsolatedComposeBoundary(composeProject: string): string {
  const owner = dockerOutput(["ps", "--filter", "publish=3000", "--format", "{{.Label \"com.docker.compose.project\"}}|{{.Label \"com.docker.compose.service\"}}"])
    .split(/\r?\n/).filter(Boolean);
  if (owner.length !== 1 || owner[0] !== `${composeProject}|web`) {
    throw new Error("loopback port 3000 is not owned by the explicitly isolated LedgerSync web service");
  }
  const postgres = dockerOutput([
    "ps",
    "--filter", `label=com.docker.compose.project=${composeProject}`,
    "--filter", "label=com.docker.compose.service=postgres",
    "--format", "{{.ID}}",
  ]).split(/\r?\n/).filter(Boolean);
  if (postgres.length !== 1 || !/^[0-9a-f]{12,64}$/i.test(postgres[0])) {
    throw new Error("the explicitly isolated LedgerSync PostgreSQL service is not uniquely running");
  }
  return postgres[0];
}

export function requireIsolatedRealStack(): RealStackRun {
  const rawBaseURL = process.env.LEDGERSYNC_SYSTEM_WEB_URL ?? "";
  const sourceAccountID = process.env.LEDGERSYNC_SYSTEM_SEEDED_SOURCE_ACCOUNT_ID ?? "";
  const runID = process.env.LEDGERSYNC_SYSTEM_RUN_ID ?? "";
  const rawComposeProject = process.env.LEDGERSYNC_SYSTEM_COMPOSE_PROJECT ?? "";
  if (process.env.LEDGERSYNC_SYSTEM_ALLOW_LEDGER_MUTATION !== "true" || process.env.LEDGERSYNC_SYSTEM_ISOLATED_PROJECT !== "true" || !rawBaseURL || !uuid.test(sourceAccountID) || !/^[a-z0-9-]{3,32}$/.test(runID)) {
    throw new Error("real-stack lifecycle tests require explicit isolated-project mutation approval, loopback web URL, seeded source account UUID, and a bounded unique run ID");
  }
  const baseURL = parseSystemWebURL(rawBaseURL);
  const composeProject = parseIsolatedComposeProject(rawComposeProject);
  const postgresContainerID = requireIsolatedComposeBoundary(composeProject);
  return { baseURL, composeProject, postgresContainerID, sourceAccountID, runID };
}

export function readComposeDurableEvidence(run: RealStackRun, expected: DurableEvidenceExpectation): DurableEvidence {
  for (const id of [expected.accountID, expected.fundingTransferID, expected.returnTransferID, expected.reconciliationRunID]) {
    if (!uuid.test(id)) throw new Error("durable evidence identifiers must be UUIDs");
  }
  for (const key of [expected.createIdempotencyKey, expected.deniedCloseIdempotencyKey, expected.fundingIdempotencyKey, expected.returnIdempotencyKey, expected.reconciliationIdempotencyKey]) {
    if (!/^[\x21-\x7e]{16,255}$/.test(key)) throw new Error("durable evidence idempotency keys must be bounded visible ASCII");
  }
  if (!/^[A-Za-z0-9][A-Za-z0-9._-]{2,63}$/.test(expected.externalReference)) {
    throw new Error("durable evidence external reference is invalid");
  }
  const sql = `
WITH selected_transfers(id) AS (
  VALUES (:'funding_transfer_id'::uuid), (:'return_transfer_id'::uuid)
), posting_totals AS (
  SELECT j.transfer_id, count(*) AS posting_count,
         count(DISTINCT p.account_id) AS account_count,
         sum(CASE WHEN p.direction='debit' THEN p.amount_minor ELSE 0 END) AS debit_minor,
         sum(CASE WHEN p.direction='credit' THEN p.amount_minor ELSE 0 END) AS credit_minor
  FROM journal_transactions j
  JOIN ledger_postings p ON p.journal_transaction_id=j.id
  WHERE j.transfer_id IN (SELECT id FROM selected_transfers)
  GROUP BY j.transfer_id
)
SELECT json_build_object(
  'account_count', (SELECT count(*) FROM accounts WHERE id=:'account_id'::uuid AND external_reference=:'external_reference'),
  'owner_count', (SELECT count(*) FROM account_owners WHERE account_id=:'account_id'::uuid AND permission='debit'),
  'zero_closed_projection_count', (SELECT count(*) FROM accounts a JOIN account_balance_projections p ON p.account_id=a.id WHERE a.id=:'account_id'::uuid AND a.status='closed' AND p.available_minor=0 AND p.ledger_minor=0),
  'create_idempotency_count', (SELECT count(*) FROM idempotency_requests WHERE operation='accounts.create.v1' AND idempotency_key=:'create_key' AND state='completed' AND response_status=201 AND response_body->>'account_id'=:'account_id' AND transfer_id IS NULL),
  'denied_close_idempotency_count', (SELECT count(*) FROM idempotency_requests WHERE operation='accounts.update.v1' AND idempotency_key=:'denied_close_key' AND state='failed' AND response_status=422 AND response_body->>'error_code'='non_zero_balance'),
  'successful_account_audit_count', (SELECT count(*) FROM audit_events WHERE target_id=:'account_id' AND outcome='succeeded' AND event_type IN ('account.created','account.status_changed')),
  'denied_close_audit_count', (SELECT count(*) FROM audit_events WHERE target_id=:'account_id' AND outcome='denied' AND event_type='account.command_denied' AND sanitized_metadata->>'denial_code'='non_zero_balance'),
  'account_outbox_count', (SELECT count(*) FROM outbox_events WHERE aggregate_type='account' AND aggregate_id=:'account_id'::uuid AND event_type IN ('account.created.v1','account.status.changed.v1')),
  'transfer_count', (SELECT count(*) FROM transfers WHERE id IN (SELECT id FROM selected_transfers) AND status='posted' AND amount_minor=100 AND currency='INR'),
  'transfer_idempotency_count', (SELECT count(*) FROM idempotency_requests WHERE operation='transfers.create.v1' AND idempotency_key IN (:'funding_key',:'return_key') AND transfer_id IN (SELECT id FROM selected_transfers) AND state='completed' AND response_status=201),
  'transfer_audit_count', (SELECT count(*) FROM audit_events WHERE target_id IN (SELECT id::text FROM selected_transfers) AND event_type='transfer.posted' AND outcome='succeeded'),
  'transfer_outbox_count', (SELECT count(*) FROM outbox_events WHERE transfer_id IN (SELECT id FROM selected_transfers) AND event_type='account.balance.changed.v1'),
  'posting_count', (SELECT COALESCE(sum(posting_count),0) FROM posting_totals),
  'balanced_transfer_count', (SELECT count(*) FROM posting_totals WHERE posting_count=2 AND account_count=2 AND debit_minor=credit_minor AND debit_minor=100),
  'reconciliation_count', (SELECT count(*) FROM reconciliation_runs WHERE id=:'reconciliation_run_id'::uuid AND status='matched' AND mismatch_count=0),
  'reconciliation_idempotency_count', (SELECT count(*) FROM idempotency_requests WHERE operation='reconciliation.run.v1' AND idempotency_key=:'reconciliation_key' AND state='completed' AND response_status=201 AND response_body->>'ID'=:'reconciliation_run_id'),
  'reconciliation_request_audit_count', (SELECT count(*) FROM audit_events WHERE target_id=:'reconciliation_run_id' AND event_type='reconciliation.requested' AND outcome='allowed'),
  'reconciliation_completed_audit_count', (SELECT count(*) FROM audit_events WHERE target_id=:'reconciliation_run_id' AND event_type='reconciliation.completed' AND outcome='succeeded'),
  'active_reconciliation_command_count', (SELECT count(*) FROM reconciliation_run_commands)
)::text;`;
  const variableArguments = [
    ["account_id", expected.accountID],
    ["external_reference", expected.externalReference],
    ["create_key", expected.createIdempotencyKey],
    ["denied_close_key", expected.deniedCloseIdempotencyKey],
    ["funding_transfer_id", expected.fundingTransferID],
    ["return_transfer_id", expected.returnTransferID],
    ["funding_key", expected.fundingIdempotencyKey],
    ["return_key", expected.returnIdempotencyKey],
    ["reconciliation_run_id", expected.reconciliationRunID],
    ["reconciliation_key", expected.reconciliationIdempotencyKey],
  ].flatMap(([name, value]) => ["-v", `${name}=${value}`]);
  const output = dockerOutput([
    "exec", "-i", run.postgresContainerID,
    "psql", "-X", "-v", "ON_ERROR_STOP=1", "-U", "ledgersync", "-d", "ledgersync", "-At",
    ...variableArguments,
  ], sql);
  try {
    return JSON.parse(output) as DurableEvidence;
  } catch {
    throw new Error("isolated PostgreSQL durability evidence was not one JSON object");
  }
}

export async function waitForMutationRequest(page: Page, pathname: RegExp, action: () => Promise<void>): Promise<Request> {
  const request = page.waitForRequest((candidate) => candidate.method() !== "GET" && pathname.test(new URL(candidate.url()).pathname));
  await action();
  return request;
}

export async function replayCapturedMutation(page: Page, request: Request): Promise<APIResponse> {
  const headers = request.headers();
  const replayHeaders: Record<string, string> = {
    "Content-Type": headers["content-type"] ?? "application/json",
    Origin: headers.origin ?? new URL(page.url()).origin,
  };
  for (const name of ["x-csrf-token", "idempotency-key"]) {
    if (headers[name]) replayHeaders[name] = headers[name];
  }
  expect(replayHeaders["idempotency-key"]).toBeTruthy();
  const response = await page.request.fetch(request.url(), {
    method: request.method(),
    headers: replayHeaders,
    data: request.postDataBuffer(),
    failOnStatusCode: false,
  });
  return response;
}

export function extractAccountID(url: string): string {
  const match = new URL(url).pathname.match(/^\/accounts\/([^/]+)$/);
  if (!match) throw new Error(`account detail URL did not contain an account ID: ${url}`);
  return decodeURIComponent(match[1]);
}
