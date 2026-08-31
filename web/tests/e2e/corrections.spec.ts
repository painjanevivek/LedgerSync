import { expect, test, type Route } from "@playwright/test";

import type { TransferCorrection } from "../../src/lib/api/corrections";

import {
  destinationAccount,
  mockOperatorConsole,
  sourceAccount,
  transfer,
} from "./fixtures";

const correction = {
  correction_id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
  original_transfer_id: transfer.transfer_id,
  original_journal_id: transfer.journal_transaction_id,
  requester_subject_id: "requester-1",
  debit_account_id: sourceAccount.account_id,
  credit_account_id: destinationAccount.account_id,
  amount_minor: transfer.amount_minor,
  currency: transfer.currency,
  reason_code: "operational_error",
  operator_note: "Verified duplicate settlement evidence.",
  status: "requested",
  policy_version: "transfer-correction-v1",
  control_mode: "production_dual_control",
  step_up_required: true,
  approval_expires_at: "2026-08-20T12:00:00Z",
  requested_at: "2026-08-19T12:00:00Z",
  updated_at: "2026-08-19T12:00:00Z",
} as const;

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

async function authorizeCorrections(
  page: Parameters<typeof mockOperatorConsole>[0],
) {
  await page.route("**/api/session", (route) =>
    json(route, {
      subject_id: "approver-2",
      tenant_id: "tenant-1",
      csrf_token: "csrf-test-token",
      environment: "production",
      tenant_label: "Meridian Labs · Test",
      operator_label: "Independent approver",
      scopes: [
        "accounts:read",
        "transactions:read",
        "transfers:read",
        "transfers:write",
        "events:read",
        "explainability:read",
        "reconciliation:read",
        "corrections:read",
        "corrections:write",
        "corrections:approve",
      ],
    }),
  );
}

test("independent approver reviews and posts one paired compensation", async ({
  page,
}) => {
  let current: TransferCorrection = correction;
  let postRequests = 0;
  let postKey = "";
  await mockOperatorConsole(page);
  await authorizeCorrections(page);
  await page.route("**/api/transfer-corrections?*", (route) =>
    json(route, { events: [correction] }),
  );
  await page.route(
    `**/api/transfer-corrections/${correction.correction_id}`,
    (route) => json(route, current),
  );
  await page.route(
    `**/api/transfer-corrections/${correction.correction_id}/approve`,
    (route) => {
      current = {
        ...correction,
        status: "approved",
        approver_subject_id: "approver-2",
        decision_reason: "Independent evidence review completed.",
      };
      return json(route, current);
    },
  );
  await page.route(
    `**/api/transfer-corrections/${correction.correction_id}/post`,
    (route) => {
      postRequests += 1;
      postKey = route.request().headers()["idempotency-key"] ?? "";
      current = {
          ...correction,
          status: "posted",
          approver_subject_id: "approver-2",
          decision_reason: "Independent evidence review completed.",
          compensation_transfer_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
          compensation_journal_id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
      };
      return json(route, {
        event: current,
        replayed: false,
      });
    },
  );

  await page.goto("/corrections");
  await expect(
    page.getByRole("heading", { name: "Transfer corrections", exact: true }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Open control record" }).click();
  await expect(page.getByText("Original · permanent")).toBeVisible();
  await expect(page.getByText("Compensation · additive")).toBeVisible();
  await page
    .getByLabel("Decision or cancellation reason")
    .fill("Independent evidence review completed.");
  await page.getByRole("button", { name: "Approve request" }).click();
  const postTrigger = page.getByRole("button", { name: "Review reverse-transfer posting" });
  await expect(postTrigger).toBeVisible();
  await postTrigger.click();
  await expect(page.getByRole("heading", { name: "Post exact reverse transfer?" })).toBeFocused();
  await expect(page.getByText("Create one additive balanced reverse transfer")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(postTrigger).toBeFocused();
  await postTrigger.click();
  await page.getByRole("button", { name: "Post exact reverse transfer", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Posted reverse transfer" }),
  ).toBeVisible();
  await expect(
    page.getByText("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"),
  ).toBeVisible();
  expect(postRequests).toBe(1);
  expect(postKey.length).toBeGreaterThanOrEqual(16);
});

test("a sensitive correction request turns step-up failure into a safe return path", async ({
  page,
}) => {
  await mockOperatorConsole(page);
  await authorizeCorrections(page);
  await page.route(
    `**/api/transfers/${transfer.transfer_id}/corrections`,
    (route) => json(route, { error: { code: "step_up_required" } }, 428),
  );
  await page.goto(`/transfers/${transfer.transfer_id}`);
  await page.getByText("Start correction request").click();
  await page
    .getByLabel("Verified operator note")
    .fill("Verified duplicate settlement evidence.");
  await page.getByRole("button", { name: "Record correction request" }).click();
  const reauthenticate = page.getByRole("link", { name: "Reauthenticate" });
  await expect(reauthenticate).toBeVisible();
  await expect(reauthenticate).toHaveAttribute(
    "href",
    /prompt=login.*return_to=/,
  );
  await expect(
    page.getByText(/No correction request was assumed to succeed/),
  ).toBeVisible();
});
