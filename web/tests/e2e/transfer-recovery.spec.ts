import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { mockOperatorConsole, sourceAccount, destinationAccount } from "./fixtures";

const reference="50000000-0000-4000-8000-000000000001";
const saved={request_reference:reference,source_account_id:sourceAccount.account_id,destination_account_id:destinationAccount.account_id,currency:"INR",amount_minor:"100",created_at:"2026-10-10T12:00:00Z",state:"unresolved"};

test("confirmed detailed transfer stays visible when browser retry cleanup fails",async({page})=>{
 await mockOperatorConsole(page);
 await page.addInitScript(()=>{Storage.prototype.removeItem=()=>{throw new DOMException("Storage blocked","SecurityError");};});
 let posts=0;
 await page.route("**/api/transfers",route=>{posts++;return route.fulfill({status:201,json:{transfer_id:"33333333-3333-4333-8333-333333333333",status:"posted",currency:"INR",amount_minor:"100",balances:{}}});});
 await page.goto("/transfers");
 await page.getByText("Create a transfer",{exact:true}).click();
 await page.getByLabel("Amount").fill("1.00");
 await page.getByRole("button",{name:"Review transfer",exact:true}).click();
 await page.getByRole("button",{name:"Confirm and post",exact:true}).click();
 await expect(page.getByRole("heading",{name:"Transfer posted",exact:true})).toBeVisible();
 await expect(page.getByRole("button",{name:"Prepare another transfer"})).toBeDisabled();
 await expect(page.getByText(/completed, but its local retry information could not be cleared/)).toBeVisible();
 expect(posts).toBe(1);
});

test("saved recovery requires exact review, sends no replacement body, and opens receipt",async({page})=>{
 await mockOperatorConsole(page);
 await page.route("**/api/transfers/recovery",route=>route.fulfill({json:{requests:[saved]}}));
 let writes=0;
 await page.route(`**/api/transfers/recovery/${reference}`,route=>{writes++;expect(route.request().postData()).toBeNull();expect(route.request().headers()["idempotency-key"]).toBeUndefined();return route.fulfill({status:201,json:{transfer_id:"transfer-existing",status:"posted",amount_minor:"100",currency:"INR"}});});
 await page.goto("/transfers#saved-transfer-recovery");
 await page.getByRole("button",{name:`Review saved request ${reference}`}).click();
 const dialog=page.getByRole("dialog",{name:"Recover the original transfer?"});
 await expect(dialog.getByText("INR 1.00",{exact:true})).toBeVisible();
 expect(writes).toBe(0);
 await dialog.getByRole("button",{name:"Cancel recovery"}).click();
 await expect(page.getByRole("button",{name:`Review saved request ${reference}`})).toBeFocused();
 await page.getByRole("button",{name:`Review saved request ${reference}`}).click();
 await dialog.getByRole("button",{name:"Retry original request"}).click();
 await expect(page.getByRole("heading",{name:"Recovered transfer posted"})).toBeVisible();expect(writes).toBe(1);
 await page.getByRole("link",{name:"Open recovered receipt"}).click();await expect(page).toHaveURL(/\/transfers\/transfer-existing/);
});

test("a known receipt is read without posting and reflows at 320 pixels",async({page})=>{
 await mockOperatorConsole(page);await page.route("**/api/transfers/recovery",route=>route.fulfill({json:{requests:[{...saved,state:"posted",transfer_id:"transfer-existing"}]}}));
 let writes=0;page.on("request",request=>{if(request.method()==="POST")writes++;});
 await page.setViewportSize({width:320,height:900});await page.goto("/transfers#saved-transfer-recovery");
 await expect(page.getByRole("link",{name:"Open original posted receipt"})).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
 const scan=await new AxeBuilder({page}).include('[aria-label="Saved transfer requests"]').analyze();expect(scan.violations).toEqual([]);
 await page.getByRole("link",{name:"Open original posted receipt"}).click();expect(writes).toBe(0);
});

test("read-only recovery cannot submit and failed refresh cannot mean empty scope",async({page})=>{
 await mockOperatorConsole(page);await page.route("**/api/session",route=>route.fulfill({json:{subject_id:"operator-1",tenant_id:"tenant-1",csrf_token:"csrf-test-token",scopes:["transfers:read","accounts:read"]}}));
 await page.route("**/api/transfers/recovery",route=>route.fulfill({json:{requests:[saved]}}));await page.goto("/transfers#saved-transfer-recovery");
 await expect(page.getByRole("button",{name:`Review saved request ${reference}`})).toBeDisabled();
 await page.route("**/api/transfers/recovery",route=>route.fulfill({status:503,json:{error:{code:"temporary_unavailable"}}}));
 await page.getByRole("button",{name:"Refresh saved requests"}).click();await expect(page.getByRole("heading",{name:"Saved requests unavailable"})).toBeVisible();
 await expect(page.getByRole("heading",{name:"No saved requests in this page"})).toHaveCount(0);
});

test("a failed durable prepare cannot send a posting command",async({page})=>{
 await mockOperatorConsole(page);let posts=0;await page.route("**/api/transfers",route=>{posts++;return route.abort();});
 await page.route("**/api/transfers/recovery",route=>route.fulfill({status:503,json:{error:{code:"temporary_unavailable"}}}));
 await page.goto("/transfers/new");await page.getByLabel("Amount").fill("1.00");await page.getByRole("button",{name:"Review transfer"}).click();await page.getByRole("button",{name:"Confirm transfer"}).click();
 await expect(page.getByText(/No posting command was sent in this attempt/)).toBeVisible();expect(posts).toBe(0);
});
