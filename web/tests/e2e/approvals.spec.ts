import { expect, test } from "@playwright/test";

import { approvalItems, mockOperatorConsole } from "./fixtures";

test.beforeEach(async ({ page }) => {
  await mockOperatorConsole(page);
});

test("approval inbox keeps typed evidence, bounded counts, and exact return context", async ({ page }) => {
  await page.goto("/approvals?domain=funding&actionable_by_me=true");

  await expect(page.getByRole("heading", { name: "Approvals", exact: true })).toBeVisible();
  await expect(page.getByText("2 records on this page. A total is not calculated or implied.")).toBeVisible();
  await expect(page.getByText("12d old")).toBeVisible();
  await expect(page.getByText("Self-approval blocked")).toBeVisible();
  await expect(page.getByRole("link", { name: "Review decision" })).toHaveAttribute(
    "href",
    `/funding/${approvalItems[0].record_id}?return_to=%2Fapprovals%3Fdomain%3Dfunding%26actionable_by_me%3Dtrue`,
  );

  await page.getByRole("link", { name: "Review decision" }).click();
  await expect(page.getByRole("link", { name: "Back to approvals" })).toHaveAttribute(
    "href",
    "/approvals?domain=funding&actionable_by_me=true",
  );
});

test("approval filters persist in the URL and are sent as typed server filters", async ({ page }) => {
  let requestedURL = "";
  await page.unroute("**/api/approvals?*");
  await page.route("**/api/approvals?*", (route) => {
    requestedURL = route.request().url();
    return route.fulfill({ status:200,contentType:"application/json",body:JSON.stringify({ items:approvalItems,page_count:approvalItems.length }) });
  });
  await page.goto("/approvals");

  await page.getByLabel("Approval domain").selectOption("correction");
  await page.getByLabel("Exact domain status").selectOption("correction:requested");
  await page.getByLabel("Requester subject").fill("operator-1");
  await page.getByLabel("Age").selectOption("over_7d");
  await page.getByLabel("Actionable by me only").check();
  await page.getByRole("button", { name: "Apply filters" }).click();

  await expect(page).toHaveURL(/domain=correction/);
  await expect(page).toHaveURL(/status=correction%3Arequested/);
  await expect.poll(() => requestedURL).toContain("domain=correction");
  expect(requestedURL).toContain("status=correction%3Arequested");
  expect(requestedURL).toContain("requester=operator-1");
  expect(requestedURL).toContain("age=over_7d");
  expect(requestedURL).toContain("actionable_by_me=true");
});

test("approval denial does not request protected evidence", async ({ page }) => {
  let requested = false;
  await page.unroute("**/api/session");
  await page.route("**/api/session", (route) => route.fulfill({ status:200,contentType:"application/json",body:JSON.stringify({ subject_id:"reader",tenant_id:"tenant-1",csrf_token:"csrf",scopes:["accounts:read"],environment:"production" }) }));
  await page.unroute("**/api/approvals?*");
  await page.route("**/api/approvals?*", (route) => { requested = true; return route.fulfill({ status:403,body:"{}" }); });

  await page.goto("/approvals");

  await expect(page.getByText("Approval authority required")).toBeVisible();
  expect(requested).toBe(false);
});

test("approval unavailability is never presented as an empty queue", async ({ page }) => {
  await page.unroute("**/api/approvals?*");
  await page.route("**/api/approvals?*", (route) => route.fulfill({ status:503,contentType:"application/json",headers:{"X-Request-ID":"approval-outage-1"},body:JSON.stringify({ error:{ code:"temporary_unavailable" } }) }));

  await page.goto("/approvals");

  await expect(page.getByText("Approval evidence unavailable")).toBeVisible();
  await expect(page.getByText("No approvals match these filters")).toHaveCount(0);
});

test("invalid shared approval queries fail before protected evidence is requested", async ({ page }) => {
  let requested = false;
  await page.unroute("**/api/approvals?*");
  await page.route("**/api/approvals?*", (route) => {
    requested = true;
    return route.fulfill({ status: 500, body: "{}" });
  });

  await page.goto("/approvals?domain=funding&domain=correction");

  await expect(page.getByText("Invalid approval investigation URL")).toBeVisible();
  await expect(page.getByText("No protected approval request was made.")).toBeVisible();
  expect(requested).toBe(false);
  await page.getByRole("button", { name: "Clear invalid filters" }).click();
  await expect(page).toHaveURL(/\/approvals$/);
});
