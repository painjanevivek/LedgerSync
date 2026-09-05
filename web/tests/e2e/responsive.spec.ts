import { expect, test } from "@playwright/test";

import { fundingEvent, mockOperatorConsole, sourceAccount, transfer } from "./fixtures";

const viewports=[{width:320,height:640},{width:360,height:800},{width:390,height:844},{width:768,height:1024},{width:1024,height:768},{width:1280,height:800},{width:1440,height:900},{width:1920,height:1080},{width:2560,height:1440}];

for(const viewport of viewports){
  test(`core evidence journeys reflow at ${viewport.width}x${viewport.height}`,async({page})=>{
    await page.setViewportSize(viewport); await page.context().grantPermissions(["clipboard-read","clipboard-write"]); await mockOperatorConsole(page, { experienceMode: "simple" }); await page.goto("/");
    await expect(page.getByRole("heading",{name:"Your money at a glance"})).toBeVisible();
    const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>document.documentElement.clientWidth);expect(overflow).toBe(false);
    if(viewport.width<1280){await page.getByRole("button",{name:/menu/i}).click();}
    await page.getByRole("link",{name:"Accounts", exact:true}).click();
    await expect(page.locator("strong").filter({hasText:"Operating Reserve"}).first()).toBeVisible();
    await page.goto(`/accounts/${sourceAccount.account_id}`); await expect(page.getByRole("heading",{name:"Operating Reserve",exact:true})).toBeVisible();
    await page.goto(`/transfers/${transfer.transfer_id}`); await expect(page.getByRole("heading", { name: "Transfer completed" })).toBeVisible();
    await page.goto("/tasks"); await expect(page.getByRole("heading", { name: /items need attention/ })).toBeVisible();
    expect(await page.evaluate(()=>document.documentElement.scrollWidth>document.documentElement.clientWidth)).toBe(false);
  });
}

test("funding uses the full operator workspace without widening exact-value controls", async ({ page }) => {
  await mockOperatorConsole(page);
  const requiredViewports = [
    { width: 390, height: 844 },
    { width: 768, height: 1024 },
    { width: 1024, height: 768 },
    { width: 1280, height: 800 },
    { width: 1440, height: 900 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ];

  for (const viewport of requiredViewports) {
    await page.setViewportSize(viewport);
    await page.goto("/funding");
    await expect(page.getByRole("heading", { name: "Funding records", exact: true })).toBeVisible();
    const layout = await page.evaluate(() => {
      const canvas = document.querySelector<HTMLElement>(".console-main")!.getBoundingClientRect();
      const workspace = document.querySelector<HTMLElement>(".operator-workspace")!.getBoundingClientRect();
      const primary = document.querySelector<HTMLElement>(".operator-workspace-primary")!.getBoundingClientRect();
      const rail = document.querySelector<HTMLElement>(".operator-workspace-rail")!.getBoundingClientRect();
      return {
        canvasCenter: canvas.left + canvas.width / 2,
        workspaceCenter: workspace.left + workspace.width / 2,
        horizontalOverflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
        primaryWidth: primary.width,
        railVisible: rail.width > 0 && rail.height > 0,
        railStartsAfterPrimary: rail.left >= primary.right,
      };
    });
    expect(layout.horizontalOverflow).toBe(false);
    expect(Math.abs(layout.workspaceCenter - layout.canvasCenter)).toBeLessThanOrEqual(1);
    expect(layout.railVisible).toBe(viewport.width >= 1600);
    if (viewport.width >= 1600) expect(layout.railStartsAfterPrimary).toBe(true);
  }

  await page.setViewportSize({ width: 2560, height: 1440 });
  await page.goto("/funding");
  await page.getByRole("button", { name: "Record funding" }).click();
  await expect(page.getByRole("heading", { name: "Add a funding record", exact: true })).toBeVisible();
  const controls = await page.locator(".funding-evidence-form input, .funding-evidence-form select").evaluateAll((elements) => elements.map((element) => element.getBoundingClientRect().width));
  expect(Math.max(...controls)).toBeLessThanOrEqual(600);
  expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
  await expect(page.getByText(fundingEvent.external_reference, { exact: true })).toBeVisible();
});

test("funding collapses its contextual rail at zoom-equivalent reflow widths", async ({ page }) => {
  await mockOperatorConsole(page);
  for (const viewport of [{ width: 640, height: 720 }, { width: 320, height: 640 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/funding");
    await page.getByRole("button", { name: "Record funding" }).click();
    await expect(page.getByRole("heading", { name: "Add a funding record", exact: true })).toBeVisible();
    expect(await page.locator(".operator-workspace-rail").isVisible()).toBe(false);
    expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
    await expect(page.locator("#funding-amount")).toBeVisible();
  }
});

test("funding explains why its four inputs are required", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto("/funding");
  await page.getByRole("button", { name: "Record funding" }).click();
  const fields = ["#funding-destination-account", "#funding-amount", "#funding-reference", "#funding-supporting-document"];
  for (const selector of fields) {
    const field = page.locator(selector);
    await expect(field).toBeVisible();
    await expect(field).toHaveAttribute("required", "");
  }
  await expect(page.getByText("Why all four?", { exact: true })).toBeVisible();
  await expect(page.getByText("Another operator needs them to check the record before your balance can change.")).toBeVisible();
});

test("shared field labels show the server-backed required or optional state", async ({ page }) => {
  await mockOperatorConsole(page);

  await page.goto("/accounts/new");
  await expect(page.locator(".account-command-form .field-requirement.required")).toHaveCount(3);
  await expect(page.getByLabel("Display name")).toHaveAttribute("required", "");
  await expect(page.getByLabel("External reference")).toHaveAttribute("required", "");
  await expect(page.getByLabel("Category")).toHaveAttribute("required", "");

  await page.goto("/transfers/new");
  await expect(page.locator(".transfer-form .field-requirement.required")).toHaveCount(3);
  await expect(page.getByLabel("From account")).toHaveAttribute("required", "");
  await expect(page.getByLabel("To account")).toHaveAttribute("required", "");
  await expect(page.getByLabel("Amount")).toHaveAttribute("required", "");

  await page.goto("/events");
  await expect(page.locator(".event-filter-document .field-requirement.optional")).toHaveCount(7);
  await expect(page.getByLabel("Event type")).not.toHaveAttribute("required", "");
});

test("approval filters and exact decision evidence remain usable from compact to ultrawide", async ({ page }) => {
  await mockOperatorConsole(page);
  for (const viewport of [{ width:320,height:640 }, ...viewports]) {
    await page.setViewportSize(viewport);
    await page.goto("/approvals");
    await expect(page.getByRole("heading", { name:"Approvals",exact:true })).toBeVisible();
    await expect(page.getByRole("button", { name:"Apply filters" })).toBeVisible();
    await expect(page.getByRole("region", { name:"Approval queue records" })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
  }
});
