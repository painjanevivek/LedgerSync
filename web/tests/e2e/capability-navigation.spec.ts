import { expect, test, type Page, type Route } from "@playwright/test";

import { mockOperatorConsole } from "./fixtures";

function json(route: Route, body: unknown) {
  return route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

async function useSession(
  page: Page,
  environment: "local" | "production",
  scopes: string[],
) {
  await page.route("**/api/session", (route) =>
    json(route, {
      subject_id: "capability-operator",
      tenant_id: "tenant-1",
      csrf_token: "csrf-test-token",
      scopes,
      environment,
      tenant_label: environment === "local" ? "Local capability workspace" : "Production tenant",
      operator_label: "Scoped operator",
    }),
  );
}

function captureAPIPaths(page: Page) {
  const paths: string[] = [];
  page.on("request", (request) => {
    const path = new URL(request.url()).pathname;
    if (path.startsWith("/api/")) paths.push(path);
  });
  return paths;
}

test("local finance operator sees grouped work, investigation, platform, and environment navigation", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/accounts");
  await expect(page.getByRole("heading", { name: "Accounts", exact: true })).toBeVisible();

  const navigation = page.getByRole("navigation", { name: "Primary navigation" });
  await expect(navigation.getByText("Work", { exact: true })).toBeVisible();
  await expect(navigation.getByText("Investigate", { exact: true })).toBeVisible();
  await expect(navigation.getByText("Platform", { exact: true })).toBeVisible();
  await expect(navigation.getByText("Environment", { exact: true })).toBeVisible();
  await expect(navigation.getByRole("link", { name: "Approvals" })).toBeVisible();
  await navigation.getByText("Investigate", { exact: true }).click();
  await expect(navigation.getByRole("link", { name: "Events & webhooks" })).toBeVisible();
  await navigation.getByText("Environment", { exact: true }).click();
  await expect(navigation.getByRole("link", { name: "Local status" })).toBeVisible();
});

test("production read role sees only relevant navigation and never local status", async ({ page }) => {
  await mockOperatorConsole(page);
  await useSession(page, "production", ["accounts:read", "events:read"]);
  await page.goto("/accounts");
  await expect(page.getByRole("heading", { name: "Accounts", exact: true })).toBeVisible();

  const navigation = page.getByRole("navigation", { name: "Primary navigation" });
  await expect(navigation.getByRole("link", { name: "Accounts" })).toBeVisible();
  await navigation.getByText("Investigate", { exact: true }).click();
  await expect(navigation.getByRole("link", { name: "Events & webhooks" })).toBeVisible();
  await expect(navigation.getByRole("link", { name: "Approvals" })).toHaveCount(0);
  await expect(navigation.getByRole("link", { name: "Local status" })).toHaveCount(0);
  await expect(navigation.getByRole("link", { name: "Developer" })).toHaveCount(0);
  await expect(navigation.getByText("Platform", { exact: true })).toHaveCount(0);
  await expect(navigation.getByText("Environment", { exact: true })).toHaveCount(0);
});

test("production direct local-status access is denied without requesting diagnostics", async ({ page }) => {
  await mockOperatorConsole(page);
  await useSession(page, "production", ["local:read"]);
  const paths = captureAPIPaths(page);
  await page.goto("/local-status");
  await expect(page.getByRole("heading", { name: "Local status", exact: true })).toBeVisible();
  await expect(page.getByText("Local diagnostics not authorized")).toBeVisible();
  expect(paths).not.toContain("/api/local/diagnostics");
});

test("missing account scope denies a direct route without starting protected reads", async ({ page }) => {
  await mockOperatorConsole(page);
  await useSession(page, "production", []);
  const paths = captureAPIPaths(page);
  await page.goto("/accounts");
  await expect(page.getByText("Account read authority required")).toBeVisible();
  expect(paths).toEqual(["/api/session"]);
});

test("filtered compact navigation keeps dialog focus and Escape restoration", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockOperatorConsole(page);
  await useSession(page, "production", ["accounts:read"]);
  await page.goto("/accounts");
  const menu = page.getByRole("button", { name: "Menu" });
  await menu.click();
  const navigation = page.getByRole("navigation", { name: "Primary navigation" });
  await expect(navigation).toBeVisible();
  await expect(navigation.getByRole("link", { name: "Accounts" })).toBeVisible();
  await expect(navigation.getByRole("link", { name: "Local status" })).toHaveCount(0);
  await page.keyboard.press("Escape");
  await expect(menu).toBeFocused();
});
