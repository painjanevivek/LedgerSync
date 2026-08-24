import { expect, test } from "@playwright/test";

import { mockOperatorConsole, sourceAccount, transfer, run } from "./fixtures";

const viewports=[{width:390,height:844},{width:768,height:1024},{width:1024,height:768},{width:1366,height:768},{width:1440,height:900},{width:1920,height:1080}];

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
