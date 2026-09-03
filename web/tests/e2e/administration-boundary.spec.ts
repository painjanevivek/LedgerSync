import { expect, test, type Route } from "@playwright/test";

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
}

test("invented production administration scope still receives a non-disclosing missing route", async ({ page }) => {
  let administrationAPIRequested = false;
  await page.route("**/api/session", (route) => json(route, {
    subject_id: "operator-1",
    tenant_id: "tenant-1",
    csrf_token: "csrf-test-token",
    scopes: ["administration:manage"],
    environment: "production",
    tenant_label: "Ledger tenant",
    operator_label: "Authorized operator",
  }));
  await page.route("**/api/admin/**", (route) => {
    administrationAPIRequested = true;
    return json(route, { error: { code: "should_not_exist" } }, 500);
  });

  const response = await page.goto("/admin");
  expect(response?.status()).toBe(404);
  await expect(page.getByRole("heading", { name: "Page unavailable" })).toBeVisible();
  await expect(page.getByText("This page could not be found.")).toBeVisible();
  await expect(page.getByRole("link", { name: "Administration" })).toHaveCount(0);
  await expect(page.getByText(/tenant lifecycle|operator invite|scope grant|recovery account/i)).toHaveCount(0);
  expect(administrationAPIRequested).toBe(false);
});

test("admin and an unknown route expose the same public not-found outcome", async ({ page }) => {
  const admin = await page.goto("/admin");
  const adminHeading = await page.getByRole("heading").first().textContent();
  const unknown = await page.goto("/this-route-does-not-exist");
  const unknownHeading = await page.getByRole("heading").first().textContent();
  expect(admin?.status()).toBe(404);
  expect(unknown?.status()).toBe(404);
  expect(adminHeading).toBe(unknownHeading);
});
