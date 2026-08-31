import { expect, test } from "@playwright/test";

import { mockOperatorConsole } from "./fixtures";

test("session and connectivity authority persist across console route families", async ({ page }) => {
  await mockOperatorConsole(page);
  let sessionRequests = 0;
  page.on("request", (request) => {
    if (new URL(request.url()).pathname === "/api/session") sessionRequests += 1;
  });

  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
  await page.getByRole("link", { name: "Developer" }).click();
  await expect(page.getByRole("heading", { name: "Developer", exact: true })).toBeVisible();
  await page.getByRole("link", { name: "Recovery" }).click();
  await expect(page.getByRole("heading", { name: "Recovery", exact: true })).toBeVisible();
  await page.getByRole("link", { name: "Local status" }).click();
  await expect(page.getByRole("heading", { name: "Local status", exact: true })).toBeVisible();
  await page.getByRole("link", { name: "Funding records" }).click();
  await expect(page.getByRole("heading", { name: "Funding records", exact: true })).toBeVisible();
  await page.getByRole("link", { name: "Corrections" }).click();
  await expect(page.getByRole("heading", { name: "Transfer corrections", exact: true })).toBeVisible();

  expect(sessionRequests).toBe(1);
});

test("failed sign-out keeps the proven session visible and exposes a retryable error", async ({ page }) => {
  await mockOperatorConsole(page);
  let signOutRequests = 0;
  await page.route("**/api/auth/sign-out", (route) => {
    signOutRequests += 1;
    return route.fulfill({
      status: 503,
      contentType: "application/json",
      headers: { "X-Request-ID": "sign-out-reference" },
      body: JSON.stringify({ error: { code: "temporary_unavailable" } }),
    });
  });

  await page.goto("/");
  const signOut = page.getByRole("button", { name: "Sign out" });
  await signOut.dblclick();

  await expect(page.locator("#sign-out-error")).toContainText(
    "Sign-out was not confirmed (503). The current workspace remains signed in.",
  );
  await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
  await expect(signOut).toBeEnabled();
  expect(signOutRequests).toBe(1);
});

test("confirmed sign-out clears the shared session and returns to the login layer", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route("**/api/auth/sign-out", (route) =>
    route.fulfill({ status: 204, body: "" }),
  );

  await page.goto("/");
  await page.getByRole("button", { name: "Sign out" }).click();

  await expect(page).toHaveURL(/\/sign-in$/);
  await expect(page.getByRole("heading", { name: "Your ledger starts empty." })).toBeVisible();
  await expect(page.getByRole("button", { name: "Sign out" })).toHaveCount(0);
});
