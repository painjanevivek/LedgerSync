import { expect, test, type Route } from "@playwright/test";

import type { FundingEvent } from "../../src/lib/api/funding";
import { fundingEvent, mockOperatorConsole } from "./fixtures";

function json(route: Route, body: unknown, status = 200, headers?: Record<string, string>) {
  return route.fulfill({ status, headers, contentType: "application/json", body: JSON.stringify(body) });
}

const approvedFunding = {
  ...fundingEvent,
  status: "approved" as const,
  approver_subject_id: "finance-approver-2",
  decision_reason: "External evidence independently matched.",
  updated_at: "2026-08-31T10:00:00Z",
};

test("funding posting refreshes authoritative evidence and requires an accessible confirmation", async ({ page }) => {
  let current: FundingEvent = approvedFunding;
  let postRequests = 0;
  let postKey = "";
  await mockOperatorConsole(page);
  await page.route(`**/api/funding-events/${fundingEvent.funding_event_id}`, (route) => json(route, current));
  await page.route(`**/api/funding-events/${fundingEvent.funding_event_id}/post`, (route) => {
    postRequests += 1;
    postKey = route.request().headers()["idempotency-key"] ?? "";
    current = { ...approvedFunding, status: "posted", journal_transaction_id: "journal-posted-1", balance_version: "7" };
    return json(route, { event: current, replayed: false }, 200, { "X-Request-ID": "funding-post-request-1" });
  });

  await page.goto(`/funding/${fundingEvent.funding_event_id}`);
  const trigger = page.getByRole("button", { name: "Review journal posting" });
  await trigger.click();
  const dialog = page.getByRole("dialog", { name: "Post balanced journal?" });
  await expect(dialog.getByRole("heading", { name: "Post balanced journal?" })).toBeFocused();
  await expect(dialog.getByText("Create one immutable balanced journal")).toBeVisible();
  await expect(dialog.getByText("INR 1250.00", { exact: true })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(trigger).toBeFocused();

  await trigger.click();
  await dialog.getByRole("button", { name: "Post balanced journal", exact: true }).evaluate((button: HTMLButtonElement) => {
    button.click();
    button.click();
  });
  await expect(page.getByRole("region", { name: "Act on this funding record" }).getByRole("heading", { name: "Balanced journal posted", exact: true })).toBeVisible();
  expect(postRequests).toBe(1);
  expect(postKey.length).toBeGreaterThanOrEqual(16);
});

test("unknown funding posting survives reload and retries the identical key", async ({ page }) => {
  let attempts = 0;
  const keys: string[] = [];
  let current: FundingEvent = approvedFunding;
  await mockOperatorConsole(page);
  await page.route(`**/api/funding-events/${fundingEvent.funding_event_id}`, (route) => json(route, current));
  await page.route(`**/api/funding-events/${fundingEvent.funding_event_id}/post`, (route) => {
    attempts += 1;
    keys.push(route.request().headers()["idempotency-key"] ?? "");
    if (attempts === 1) return json(route, { error: { code: "funding_outcome_unknown" } }, 504, { "X-Request-ID": "funding-unknown-1" });
    current = { ...approvedFunding, status: "posted", journal_transaction_id: "journal-posted-2", balance_version: "8" };
    return json(route, { event: current, replayed: true }, 200, { "X-Request-ID": "funding-replay-1", "Idempotent-Replay": "true" });
  });

  await page.goto(`/funding/${fundingEvent.funding_event_id}`);
  await page.getByRole("button", { name: "Review journal posting" }).click();
  await page.getByRole("button", { name: "Post balanced journal", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Posting outcome unknown" })).toBeVisible();

  await page.reload();
  await expect(page.getByRole("heading", { name: "Posting outcome unknown" })).toBeVisible();
  await page.getByRole("button", { name: "Refresh before retry" }).click();
  await page.getByRole("button", { name: "Retry same journal post" }).click();
  await expect(page.getByRole("region", { name: "Act on this funding record" }).getByRole("heading", { name: "Balanced journal posted", exact: true })).toBeVisible();
  expect(keys).toHaveLength(2);
  expect(keys[1]).toBe(keys[0]);
});

test("an unprovable successful response remains an unknown posting outcome", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route(`**/api/funding-events/${fundingEvent.funding_event_id}`, (route) => json(route, approvedFunding));
  await page.route(`**/api/funding-events/${fundingEvent.funding_event_id}/post`, (route) => json(route, { event: { status: "posted" } }, 200, { "X-Request-ID": "funding-malformed-1" }));

  await page.goto(`/funding/${fundingEvent.funding_event_id}`);
  await page.getByRole("button", { name: "Review journal posting" }).click();
  await page.getByRole("button", { name: "Post balanced journal", exact: true }).click();

  await expect(page.getByRole("heading", { name: "Posting outcome unknown" })).toBeVisible();
  await page.reload();
  await expect(page.getByRole("heading", { name: "Posting outcome unknown" })).toBeVisible();
});

test("posting fails closed before dispatch when safe retry storage is unavailable", async ({ page }) => {
  let postRequests = 0;
  await page.addInitScript(() => {
    Storage.prototype.setItem = () => {
      throw new DOMException("Storage unavailable", "QuotaExceededError");
    };
  });
  await mockOperatorConsole(page);
  await page.route(`**/api/funding-events/${fundingEvent.funding_event_id}`, (route) => json(route, approvedFunding));
  await page.route(`**/api/funding-events/${fundingEvent.funding_event_id}/post`, (route) => {
    postRequests += 1;
    return json(route, { event: approvedFunding });
  });

  await page.goto(`/funding/${fundingEvent.funding_event_id}`);
  await page.getByRole("button", { name: "Review journal posting" }).click();
  await page.getByRole("button", { name: "Post balanced journal", exact: true }).click();

  await expect(page.getByRole("heading", { name: "Funding posting not completed" })).toBeVisible();
  await expect(page.getByText(/Safe retry storage is unavailable/)).toBeVisible();
  expect(postRequests).toBe(0);
});
