import { execFileSync } from "node:child_process";
import { writeFileSync } from "node:fs";
import { expect, test, type Page, type Request } from "@playwright/test";
import { setIsolatedExperienceMode, extractAccountID, replayCapturedMutation, requireIsolatedJourney } from "./real-stack";

async function logIn(page: Page) {
  await page.goto("/sign-in");
  await page.getByRole("link", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("heading", { name: /^(Overview|Your money at a glance)$/ })).toBeVisible();
  await setIsolatedExperienceMode(page, "simple");
}

async function createAccount(page: Page, name: string, reference: string) {
  await page.goto("/accounts/new");
  await page.getByLabel("Display name").fill(name);
  await page.getByLabel("External reference").fill(reference);
  await page.getByLabel("Category").selectOption("operating");
  await page.getByRole("button", { name: "Review account", exact: true }).click();
  await expect(page.getByText("Account begins").locator("..")).toContainText("INR 0.00");
  await expect(page.getByText("This command moves").locator("..")).toContainText("No money");
  const command = page.waitForRequest((request) => request.method() === "POST" && new URL(request.url()).pathname === "/api/me/accounts");
  await page.getByRole("button", { name: "Create account", exact: true }).click();
  const captured = await command;
  const result = page.getByRole("region", { name: "Account created" });
  await expect(result).toBeVisible();
  const href = await result.getByRole("link", { name: "View account" }).getAttribute("href");
  expect(href).toBeTruthy();
  const id = extractAccountID(new URL(href!, page.url()).toString());
  const replay = await replayCapturedMutation(page, captured);
  expect(replay.status()).toBe(201);
  expect(replay.headers()["idempotent-replay"]).toBe("true");
  expect(await replay.json()).toMatchObject({ account_id: id, available_minor: "0" });
  return { id, result };
}

test("@real-stack first account, controlled funding, lost-response transfer, exact retry, and follow-up", async ({ page }) => {
  test.setTimeout(180_000);
  page.setDefaultTimeout(10_000);
  const run = requireIsolatedJourney();
  const suffix = `${run.runID}-${Date.now().toString(36)}`;
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await logIn(page);
  const source = await createAccount(page, `Journey source ${suffix}`, `src-${suffix}`);
  await expect(source.result.getByRole("link", { name: "Record funding", exact: true })).toBeVisible();
  await source.result.getByRole("link", { name: "Record funding", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Funding records", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Record funding", exact: true }).click();
  await page.getByLabel("Account").selectOption(source.id);
  await page.getByLabel("Amount").fill("2.00");
  await page.getByLabel("Reference number").fill(`fund-${suffix}`);
  await page.getByLabel("Supporting document").fill(`test-receipt-${suffix}`);
  await page.getByRole("button", { name: "Review details", exact: true }).click();
  const recordResponse = page.waitForResponse((response) => response.request().method() === "POST" && new URL(response.url()).pathname === "/api/funding-requests");
  await page.getByRole("button", { name: "Save for review", exact: true }).click();
  const saved = await recordResponse;
  expect(saved.status()).toBe(201);
  const funding = (await saved.json()).event as { funding_event_id: string; status: string };
  expect(funding.status).toBe("requested");
  console.log("Journey: funding reference recorded; balance still zero.");
  await expect(page.getByRole("heading", { name: "Funding record", exact: true })).toBeVisible();
  const beforePost = await page.request.get(`/api/accounts/${source.id}/balance`);
  expect(beforePost.status()).toBe(200);
  expect(await beforePost.json()).toMatchObject({ available_minor: "0" });
  await page.getByRole("button", { name: "Review approval", exact: true }).click();
  await page.getByLabel("Why you approve this").fill("Matched the isolated test reference and receipt. Local single-operator policy only.");
  const approveResponse = page.waitForResponse((response) => response.request().method() === "POST" && new URL(response.url()).pathname.endsWith(`/${funding.funding_event_id}/approve`));
  await page.getByRole("button", { name: "Approve evidence", exact: true }).click();
  expect((await approveResponse).status()).toBe(200);
  console.log("Journey: local approval recorded; journal still separate.");
  await expect(page.getByText("Single-operator local policy", { exact: true })).toBeVisible();
  const beforeJournal = await page.request.get(`/api/accounts/${source.id}/balance`);
  expect(await beforeJournal.json()).toMatchObject({ available_minor: "0" });
  await page.getByRole("button", { name: "Review journal posting", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Post balanced journal?" });
  await expect(dialog.getByText("INR 2.00", { exact: true })).toBeVisible();
  const postRequest = page.waitForRequest((request) => request.method() === "POST" && new URL(request.url()).pathname.endsWith(`/${funding.funding_event_id}/post`));
  await dialog.getByRole("button", { name: "Post balanced journal", exact: true }).click();
  const fundingCommand = await postRequest;
  await expect(page.getByRole("button", { name: "Reconcile reference", exact: true })).toBeVisible();
  const fundingReplay = await replayCapturedMutation(page, fundingCommand);
  expect(fundingReplay.status()).toBe(200);
  expect(fundingReplay.headers()["idempotent-replay"]).toBe("true");
  await page.getByRole("button", { name: "Reconcile reference", exact: true }).click();
  await expect(page.getByText("The journal totals match.", { exact: true })).toBeVisible();
  const funded = await page.request.get(`/api/accounts/${source.id}/balance`);
  expect(await funded.json()).toMatchObject({ available_minor: "200" });
  console.log("Journey: one funding journal posted and verified.");

  const destination = await createAccount(page, `Journey destination ${suffix}`, `dst-${suffix}`);
  await destination.result.getByRole("link", { name: "Transfer into account", exact: true }).click();
  await expect(page.getByLabel("To account")).toHaveValue(destination.id);
  await page.getByLabel("From account").selectOption(source.id);
  await page.getByLabel("Amount").fill("1.00");
  await page.getByRole("button", { name: "Review transfer", exact: true }).click();
  let original: Request | undefined;
  let committed: { transfer_id: string; status: string } | undefined;
  // Fault injection forwards the command to the real BFF/API first. Only the
  // committed response is lost; no financial response is fabricated.
  await page.route("**/api/transfers", async (route) => {
    original = route.request();
    const response = await route.fetch();
    expect(response.status()).toBe(201);
    committed = await response.json();
    await route.abort("failed");
  });
  await page.getByRole("button", { name: "Confirm transfer", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Result not yet confirmed", exact: true })).toBeVisible();
  await expect.poll(() => committed, { timeout: 10_000 }).toMatchObject({ status: "posted" });
  expect(original).toBeTruthy();
  await expect(page.getByRole("button", { name: "Retry this same request safely", exact: true })).toBeEnabled();
  await page.unroute("**/api/transfers");
  const retryResponse = page.waitForResponse((response) => response.request().method() === "POST" && new URL(response.url()).pathname === "/api/transfers");
  await page.getByRole("button", { name: "Retry this same request safely", exact: true }).click();
  const replay = await retryResponse;
  expect(replay.status()).toBe(201);
  expect(replay.headers()["idempotent-replay"]).toBe("true");
  expect(replay.request().headers()["idempotency-key"]).toBe(original!.headers()["idempotency-key"]);
  expect(replay.request().headers()["x-ledgersync-request-reference"]).toBe(original!.headers()["x-ledgersync-request-reference"]);
  expect(replay.request().postDataJSON()).toEqual(original!.postDataJSON());
  expect(await replay.json()).toMatchObject({ transfer_id: committed!.transfer_id, status: "posted", amount_minor: "100" });
  await expect(page.getByRole("heading", { name: "Transfer completed", exact: true })).toBeVisible();
  await page.goto(`/transfers/${committed!.transfer_id}`);
  await expect(page.getByRole("heading", { name: "Transfer completed", exact: true })).toBeVisible();
  await page.getByText("View transfer details", { exact: true }).click();
  await expect(page.getByText(committed!.transfer_id, { exact: true }).first()).toBeVisible();
  await page.goto(`/search?q=${committed!.transfer_id}`);
  await expect(page.getByRole("link", { name: "Open authoritative detail", exact: true })).toBeVisible();
  await page.getByRole("link", { name: "Open authoritative detail", exact: true }).click();
  await page.getByRole("link", { name: /Back to previous view/ }).click();
  await expect(page).toHaveURL(new RegExp(`/search\\?q=${committed!.transfer_id}$`));
  for (const accountID of [source.id, destination.id]) {
    const response = await page.request.get(`/api/accounts/${accountID}/balance`);
    expect(response.status()).toBe(200);
    expect(await response.json()).toMatchObject({ available_minor: "100" });
  }
  const proofSQL = `SELECT json_build_object('funding_count',(SELECT count(*) FROM funding_events WHERE destination_account_id='${source.id}' AND status='posted' AND amount_minor=200),'transfer_count',(SELECT count(*) FROM transfers WHERE id='${committed!.transfer_id}' AND status='posted' AND amount_minor=100),'posting_count',(SELECT count(*) FROM ledger_postings p JOIN journal_transactions j ON j.id=p.journal_transaction_id WHERE j.transfer_id='${committed!.transfer_id}'),'funding_journal_count',(SELECT count(*) FROM journal_transactions WHERE funding_event_id='${funding.funding_event_id}'))::text;`;
  // Every interpolated identifier comes from the verified server UUID response.
  for (const id of [source.id, destination.id, funding.funding_event_id, committed!.transfer_id]) expect(id).toMatch(/^[0-9a-f-]{36}$/i);
  const proof = JSON.parse(execFileSync("docker", ["exec", "-i", run.postgresContainerID, "psql", "-X", "-U", "ledgersync", "-d", "ledgersync", "-At", "-v", "ON_ERROR_STOP=1"], { input: proofSQL, encoding: "utf8", windowsHide: true, timeout: 15_000 }));
  expect(proof).toEqual({ funding_count: 1, transfer_count: 1, posting_count: 2, funding_journal_count: 1 });
  expect(errors).toEqual([]);
  writeFileSync("../.tmp/journey-validation/funding-journey.json", JSON.stringify({ sourceAccountID: source.id, destinationAccountID: destination.id, fundingEventID: funding.funding_event_id, transferID: committed!.transfer_id, proof }, null, 2));
});
