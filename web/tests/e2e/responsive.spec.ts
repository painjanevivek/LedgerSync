import { expect, test } from "@playwright/test";

import { fundingEvent, mockOperatorConsole, sourceAccount, transfer, run } from "./fixtures";

const viewports=[{width:390,height:844},{width:768,height:1024},{width:1024,height:768},{width:1280,height:800},{width:1440,height:900},{width:1920,height:1080},{width:2560,height:1440}];

for(const viewport of viewports){
  test(`core evidence journeys reflow at ${viewport.width}x${viewport.height}`,async({page})=>{
    await page.setViewportSize(viewport); await page.context().grantPermissions(["clipboard-read","clipboard-write"]); await mockOperatorConsole(page); await page.goto("/");
    await expect(page.getByRole("heading",{name:"Overview"})).toBeVisible();
    const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>document.documentElement.clientWidth);expect(overflow).toBe(false);
    if(viewport.width<761){await page.getByRole("button",{name:/menu/i}).click();}
    await page.getByRole("link",{name:"Accounts"}).click();
    await expect(page.locator("strong").filter({hasText:"Operating Reserve"}).first()).toBeVisible();
    await page.goto(`/accounts/${sourceAccount.account_id}`); await expect(page.getByRole("heading",{name:"Operating Reserve",exact:true})).toBeVisible();
    await page.goto(`/transfers/${transfer.transfer_id}`); await expect(page.getByText("Money is posted; delivery is retrying")).toBeVisible();
    await page.goto(`/reconciliation/${run.run_id}`); await expect(page.getByText(run.run_id,{exact:true}).first()).toBeVisible();
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
  await expect(page.getByRole("heading", { name: "Record external value", exact: true })).toBeVisible();
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
    await expect(page.getByRole("heading", { name: "Record external value", exact: true })).toBeVisible();
    expect(await page.locator(".operator-workspace-rail").isVisible()).toBe(false);
    expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
    await expect(page.getByLabel("Exact amount")).toBeVisible();
  }
});
