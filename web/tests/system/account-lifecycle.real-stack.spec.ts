import { expect, test, type Page } from "@playwright/test";

import {
  extractAccountID,
  readComposeDurableEvidence,
  replayCapturedMutation,
  requireIsolatedRealStack,
  type RealStackRun,
  waitForMutationRequest,
} from "./real-stack";

const transferAmount = "1.00";

type TransferProof = Readonly<{
  transferID: string;
  idempotencyKey: string;
  sourceAccountID: string;
  destinationAccountID: string;
}>;

type ReconciliationProof = Readonly<{
  runID: string;
  idempotencyKey: string;
}>;

async function postTransfer(page: Page, sourceAccountID: string, destinationAccountID: string): Promise<TransferProof> {
  await page.getByLabel("From account").selectOption(sourceAccountID);
  await page.getByLabel("To account").selectOption(destinationAccountID);
  await page.getByLabel("Exact amount").fill(transferAmount);
  await page.getByRole("button", { name: "Review transfer" }).click();
  await expect(page.getByRole("heading", { name: "Confirm exact transfer" })).toBeFocused();

  const responsePromise = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return response.request().method() === "POST" && url.pathname === "/api/transfers";
  });
  const requestPromise = page.waitForRequest((request) => request.method() === "POST" && new URL(request.url()).pathname === "/api/transfers");
  await page.getByRole("button", { name: "Confirm and post" }).click();
  const [request, response] = await Promise.all([requestPromise, responsePromise]);
  expect(response.status()).toBe(201);
  const result = await response.json() as { transfer_id?: unknown; status?: unknown; amount_minor?: unknown; currency?: unknown };
  expect(result).toMatchObject({ status: "posted", amount_minor: "100", currency: "INR" });
  expect(result.transfer_id).toEqual(expect.any(String));
  const idempotencyKey = request.headers()["idempotency-key"];
  expect(idempotencyKey).toMatch(/^[\x21-\x7e]{16,255}$/);
  await expect(page.getByRole("heading", { name: "Transfer posted" })).toBeFocused();
  return { transferID: result.transfer_id as string, idempotencyKey, sourceAccountID, destinationAccountID };
}

async function expectTransferInAccountHistory(page: Page, accountID: string, transferID: string) {
  await page.goto(`/accounts/${encodeURIComponent(accountID)}`);
  await expect(page.getByText(transferID, { exact: true }).first()).toBeVisible();
}

async function changeLifecycle(page: Page, action: "Freeze account" | "Reactivate account", reason: string) {
  const trigger = page.getByRole("button", { name: action });
  await expect(trigger).toBeEnabled();
  await trigger.click();
  const dialog = page.getByRole("dialog", { name: action });
  await expect(dialog).toBeVisible();
  await dialog.getByLabel("Reason").fill(reason);
  const responsePromise = page.waitForResponse((response) => response.request().method() === "PATCH" && /^\/api\/accounts\/[^/]+$/.test(new URL(response.url()).pathname));
  await dialog.getByRole("button", { name: `Confirm ${action.toLowerCase()}` }).click();
  expect((await responsePromise).status()).toBe(200);
  await expect(dialog).toBeHidden();
}

async function expectNonZeroCloseDenied(page: Page, run: RealStackRun, accountID: string): Promise<string> {
  const closeButton = page.getByRole("button", { name: "Close account" });
  await expect(closeButton).toBeEnabled();
  await expect(page.getByText("Close account requires exact zero")).toBeVisible();
  await closeButton.click();
  const dialog = page.getByRole("dialog", { name: "Close account" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText("Available balance").locator("..")).toContainText("INR 1.00");
  await expect(dialog.getByText("Ledger balance").locator("..")).toContainText("INR 1.00");
  await expect(dialog.getByRole("button", { name: "Confirm close account" })).toBeDisabled();
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toBeHidden();

  const accountResponse = await page.request.get(`${run.baseURL}/api/accounts/${accountID}`);
  expect(accountResponse.status()).toBe(200);
  const account = await accountResponse.json() as { account_version?: unknown };
  expect(account.account_version).toMatch(/^[1-9][0-9]*$/);
  const sessionResponse = await page.request.get(`${run.baseURL}/api/session`);
  expect(sessionResponse.status()).toBe(200);
  const session = await sessionResponse.json() as { csrf_token?: unknown };
  expect(session.csrf_token).toEqual(expect.any(String));

  const idempotencyKey = `account-close-nonzero-${run.runID}`;
  const request = {
    expected_version: account.account_version as string,
    target_status: "closed",
    reason: `System test ${run.runID}: reject non-zero close`,
  };
  const send = () => page.request.patch(`${run.baseURL}/api/accounts/${accountID}`, {
    headers: {
      "Content-Type": "application/json",
      Origin: run.baseURL,
      "X-CSRF-Token": session.csrf_token as string,
      "Idempotency-Key": idempotencyKey,
    },
    data: request,
    failOnStatusCode: false,
  });
  const original = await send();
  expect(original.status()).toBe(422);
  expect(await original.json()).toMatchObject({ error: { code: "account_not_zero" } });
  const replay = await send();
  expect(replay.status()).toBe(422);
  expect(await replay.json()).toMatchObject({ error: { code: "account_not_zero" } });
  return idempotencyKey;
}

async function expectPublicDurableReads(page: Page, run: RealStackRun, accountID: string, transfers: TransferProof[]) {
  const getJSON = async <T>(path: string): Promise<T> => {
    const response = await page.request.get(`${run.baseURL}${path}`);
    expect(response.status(), path).toBe(200);
    expect(response.headers()["cache-control"], path).toContain("no-store");
    return response.json() as Promise<T>;
  };
  const account = await getJSON<{
    account_id: string;
    status: string;
    available_minor: string;
    ledger_minor: string;
    audit_context: Array<{ event_type: string; outcome: string }>;
  }>(`/api/accounts/${accountID}`);
  expect(account).toMatchObject({ account_id: accountID, status: "closed", available_minor: "0", ledger_minor: "0" });
  expect(account.audit_context.filter((event) => event.event_type === "account.created" && event.outcome === "succeeded")).toHaveLength(1);
  expect(account.audit_context.filter((event) => event.event_type === "account.status_changed" && event.outcome === "succeeded")).toHaveLength(3);
  expect(account.audit_context.filter((event) => event.event_type === "account.command_denied" && event.outcome === "denied")).toHaveLength(1);

  const balance = await getJSON<{ available_minor: string; ledger_minor: string }>(`/api/accounts/${accountID}/balance`);
  expect(balance).toMatchObject({ available_minor: "0", ledger_minor: "0" });
  const history = await getJSON<{ transactions: Array<{ transfer_id: string }> }>(`/api/accounts/${accountID}/transactions?limit=100`);
  expect(new Set(history.transactions.map((entry) => entry.transfer_id))).toEqual(new Set(transfers.map((entry) => entry.transferID)));

  for (const transfer of transfers) {
    const detail = await getJSON<{
      transfer_id: string;
      financial_status: string;
      amount_minor: string;
      currency: string;
      source_account_id: string;
      destination_account_id: string;
      postings: Array<{ account_id: string; direction: string; amount_minor: string; currency: string }>;
    }>(`/api/transfers/${transfer.transferID}`);
    expect(detail).toMatchObject({
      transfer_id: transfer.transferID,
      financial_status: "posted",
      amount_minor: "100",
      currency: "INR",
      source_account_id: transfer.sourceAccountID,
      destination_account_id: transfer.destinationAccountID,
    });
    expect(detail.postings).toHaveLength(2);
    expect(detail.postings.map((posting) => posting.direction).sort()).toEqual(["credit", "debit"]);
    expect(new Set(detail.postings.map((posting) => posting.account_id))).toEqual(new Set([transfer.sourceAccountID, transfer.destinationAccountID]));
    expect(detail.postings.every((posting) => posting.amount_minor === "100" && posting.currency === "INR")).toBe(true);
  }
}

async function runAuthoritativeReconciliation(page: Page, run: RealStackRun): Promise<ReconciliationProof> {
  await page.goto("/reconciliation");
  await page.getByRole("button", { name: "Run reconciliation", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Review authoritative reconciliation scope" })).toBeFocused();

  const request = await waitForMutationRequest(page, /^\/api\/reconciliation\/runs$/, async () => {
    await page.getByRole("button", { name: "Start reconciliation" }).click();
  });
  await expect(page.getByRole("heading", { name: "Reconciliation passed" })).toBeFocused();

  const replay = await replayCapturedMutation(page, request);
  expect(replay.status()).toBe(201);
  expect(replay.headers()["idempotent-replay"]).toBe("true");
  const replayResult = await replay.json() as { run_id?: unknown; status?: unknown; mismatch_count?: unknown };
  expect(replayResult).toMatchObject({ status: "matched", mismatch_count: "0" });
  expect(replayResult.run_id).toMatch(/^[0-9a-f-]{36}$/i);
  const runID = replayResult.run_id as string;
  await expect(page.getByText(runID, { exact: true }).first()).toBeVisible();

  const detailResponse = await page.request.get(`${run.baseURL}/api/reconciliation/runs/${runID}`);
  expect(detailResponse.status()).toBe(200);
  expect(detailResponse.headers()["cache-control"]).toContain("no-store");
  expect(await detailResponse.json()).toMatchObject({ run_id: runID, status: "matched", mismatch_count: "0" });

  const idempotencyKey = request.headers()["idempotency-key"];
  expect(idempotencyKey).toMatch(/^[\x21-\x7e]{16,255}$/);
  return { runID, idempotencyKey };
}

test.describe("@real-stack account product lifecycle", () => {
  test("creates, replays, funds, freezes, reactivates, returns to zero, and closes through normal commands", async ({ page }) => {
    const run = requireIsolatedRealStack();
    const displayName = `System lifecycle ${run.runID}`;
    const externalReference = `sys-${run.runID}`;

    await page.goto("/accounts");
    await page.getByRole("link", { name: "Create account" }).click();
    await expect(page).toHaveURL(/\/accounts\/new(?:\?|$)/);
    await page.getByLabel("Display name").fill(displayName);
    await page.getByLabel("External reference").fill(externalReference);
    await page.getByLabel("Category").selectOption("operating");
    await page.getByRole("button", { name: "Continue to financial boundary" }).click();
    await expect(page.getByText("INR 0.00 · exact")).toBeVisible();
    await page.getByRole("button", { name: "Continue to review" }).click();

    const createRequest = await waitForMutationRequest(page, /^\/api\/me\/accounts$/, async () => {
      await page.getByRole("button", { name: "Create account" }).click();
    });
    const result = page.getByRole("region", { name: "Account created" });
    await expect(result).toBeVisible();
    await expect(result.getByText("INR 0.00", { exact: true })).toBeVisible();
    const detailHref = await result.getByRole("link", { name: "View account" }).getAttribute("href");
    expect(detailHref).toBeTruthy();
    const createdAccountID = extractAccountID(new URL(detailHref!, run.baseURL).toString());

    const replay = await replayCapturedMutation(page, createRequest);
    expect(replay.status()).toBe(201);
    expect(replay.headers()["idempotent-replay"]).toBe("true");
    const replayResult = await replay.json() as { account_id?: unknown };
    expect(replayResult.account_id).toBe(createdAccountID);
    const createIdempotencyKey = createRequest.headers()["idempotency-key"];
    expect(createIdempotencyKey).toMatch(/^[\x21-\x7e]{16,255}$/);

    await result.getByRole("link", { name: "Fund account" }).click();
    await expect(page).toHaveURL(new RegExp(`destination=${createdAccountID}`));
    await expect(page.getByLabel("To account")).toHaveValue(createdAccountID);
    const fundingTransfer = await postTransfer(page, run.sourceAccountID, createdAccountID);

    await expectTransferInAccountHistory(page, run.sourceAccountID, fundingTransfer.transferID);
    await expectTransferInAccountHistory(page, createdAccountID, fundingTransfer.transferID);
    const deniedCloseIdempotencyKey = await expectNonZeroCloseDenied(page, run, createdAccountID);

    await changeLifecycle(page, "Freeze account", `System test ${run.runID}: freeze before reactivation`);
    await expect(page.locator(".identity-strip").getByText(/^frozen$/i)).toBeVisible();
    await changeLifecycle(page, "Reactivate account", `System test ${run.runID}: reactivate for return transfer`);
    await expect(page.locator(".identity-strip").getByText(/^active$/i)).toBeVisible();

    await page.goto("/transfers");
    const returnTransfer = await postTransfer(page, createdAccountID, run.sourceAccountID);
    await expectTransferInAccountHistory(page, run.sourceAccountID, returnTransfer.transferID);
    await expectTransferInAccountHistory(page, createdAccountID, fundingTransfer.transferID);
    await expect(page.getByText(returnTransfer.transferID, { exact: true }).first()).toBeVisible();
    await expect(page.getByText("INR 0.00", { exact: true }).first()).toBeVisible();

    const closeButton = page.getByRole("button", { name: "Close account" });
    await expect(closeButton).toBeEnabled();
    await closeButton.click();
    const closeDialog = page.getByRole("dialog", { name: "Close account" });
    await closeDialog.getByLabel("Reason").fill(`System test ${run.runID}: verified exact zero`);
    await closeDialog.getByLabel("Confirm external reference").fill(externalReference);
    const closeResponse = page.waitForResponse((response) => response.request().method() === "PATCH" && new URL(response.url()).pathname === `/api/accounts/${createdAccountID}`);
    await closeDialog.getByRole("button", { name: "Confirm close account" }).click();
    expect((await closeResponse).status()).toBe(200);
    await expect(closeDialog).toBeHidden();
    await expect(page.locator(".identity-strip").getByText(/^closed$/i)).toBeVisible();
    await expect(page.getByText("Account lifecycle is terminal")).toBeVisible();
    await expect(page.getByText(fundingTransfer.transferID, { exact: true }).first()).toBeVisible();
    await expect(page.getByText(returnTransfer.transferID, { exact: true }).first()).toBeVisible();

    await expectPublicDurableReads(page, run, createdAccountID, [fundingTransfer, returnTransfer]);
    const reconciliation = await runAuthoritativeReconciliation(page, run);
    expect(readComposeDurableEvidence(run, {
      accountID: createdAccountID,
      externalReference,
      createIdempotencyKey,
      deniedCloseIdempotencyKey,
      fundingTransferID: fundingTransfer.transferID,
      fundingIdempotencyKey: fundingTransfer.idempotencyKey,
      returnTransferID: returnTransfer.transferID,
      returnIdempotencyKey: returnTransfer.idempotencyKey,
      reconciliationRunID: reconciliation.runID,
      reconciliationIdempotencyKey: reconciliation.idempotencyKey,
    })).toEqual({
      account_count: 1,
      owner_count: 1,
      zero_closed_projection_count: 1,
      create_idempotency_count: 1,
      denied_close_idempotency_count: 1,
      successful_account_audit_count: 4,
      denied_close_audit_count: 1,
      account_outbox_count: 4,
      transfer_count: 2,
      transfer_idempotency_count: 2,
      transfer_audit_count: 2,
      transfer_outbox_count: 4,
      posting_count: 4,
      balanced_transfer_count: 2,
      reconciliation_count: 1,
      reconciliation_idempotency_count: 1,
      reconciliation_request_audit_count: 1,
      reconciliation_completed_audit_count: 1,
      active_reconciliation_command_count: 0,
    });

    console.log([
      "PHASE3_FIXTURES",
      `run=${run.runID}`,
      `account=${createdAccountID}`,
      `funding_transfer=${fundingTransfer.transferID}`,
      `return_transfer=${returnTransfer.transferID}`,
      `reconciliation_run=${reconciliation.runID}`,
    ].join(" "));
  });
});
