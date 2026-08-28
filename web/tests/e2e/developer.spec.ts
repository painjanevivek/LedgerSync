import AxeBuilder from "@axe-core/playwright";
import { expect,test,type Page,type Route } from "@playwright/test";

import { developerMetadata,mockOperatorConsole } from "./fixtures";

function json(route:Route,body:unknown,status=200){return route.fulfill({status,contentType:"application/json",body:JSON.stringify(body)});}
async function expectAccessible(page:Page){const result=await new AxeBuilder({page}).exclude(".app-shell > aside").analyze();expect(result.violations.filter((violation)=>["serious","critical"].includes(violation.impact??""))).toEqual([]);}

test("developer sees the local boundary, distinct auth lanes, and versioned endpoint groups",async({page})=>{
  await mockOperatorConsole(page);
  await page.goto("/developer");
  await expect(page.getByRole("heading",{name:"Developer",exact:true})).toBeVisible();
  await expect(page.getByText("http://127.0.0.1:3100/api",{exact:true})).toBeVisible();
  await expect(page.getByRole("heading",{name:"Authentication"})).toBeVisible();
  await expect(page.getByRole("heading",{name:"Browser BFF session"})).toBeVisible();
  await expect(page.getByRole("heading",{name:"Private API development token"})).toBeVisible();
  await expect(page.getByText(/never paste revealed output into this browser/i)).toBeVisible();
  const createAccountRow=page.locator(".endpoint-row").filter({hasText:"createAccount"});
  await expect(createAccountRow.getByText("POST",{exact:true})).toBeVisible();
  await expect(createAccountRow.getByText("/api/accounts",{exact:true})).toBeVisible();
  const developerGroup=page.locator(".endpoint-catalogue details").filter({hasText:"Developer contracts"});
  await developerGroup.locator("summary").click();
  await expect(developerGroup.getByText("developer:read",{exact:true}).first()).toBeVisible();
  await expectAccessible(page);
});

test("exact transfer and zero-account examples preserve string money and announce non-secret copy",async({page,context})=>{
  await context.grantPermissions(["clipboard-read","clipboard-write"]);
  await mockOperatorConsole(page);
  await page.goto("/developer");
  const transfer=page.getByRole("article",{name:"Post an exact INR transfer"});
  await expect(transfer.locator(".request-proof-line.exact-money code")).toHaveText('"amount": "125.50"');
  await expect(transfer.getByText(/Never send a JSON number for money/)).toBeVisible();
  await transfer.getByRole("button",{name:"Copy Post an exact INR transfer browser example"}).click();
  await expect(transfer.getByRole("status")).toHaveText("Post an exact INR transfer browser example copied");
  const copied=await page.evaluate(()=>navigator.clipboard.readText());
  expect(copied).toContain('"amount": "125.50"');
  expect(copied).toContain('"Idempotency-Key": "example-transfer-key-0001"');
  expect(copied).not.toMatch(/Bearer|Authorization|LEDGERSYNC_/);

  const account=page.getByRole("article",{name:"Create an exact-zero INR account"});
  await expect(account.getByText('"available_minor":"0"',{exact:false})).toBeVisible();
  await expect(account.getByText('"ledger_minor":"0"',{exact:false})).toBeVisible();
  await expect(account.getByText(/No opening amount is accepted/)).toBeVisible();
});

test("retry and error evidence gives safe actions without exposing an HTTP runner",async({page})=>{
  await mockOperatorConsole(page);
  await page.goto("/developer");
  const retries=page.getByRole("region",{name:"Safe retry outcomes"});
  await expect(retries.getByText("response_unknown",{exact:true})).toBeVisible();
  await expect(retries.getByText(/Retry the identical request with the same idempotency key/).first()).toBeVisible();
  await expect(retries.getByText("idempotency_conflict",{exact:true})).toBeVisible();
  await expect(page.getByRole("button",{name:/send request/i})).toHaveCount(0);
  await expect(page.getByLabel(/arbitrary url/i)).toHaveCount(0);
  await expect(page.getByText(/raw-log search/i)).toBeVisible();
  await expect(page.getByText("X-Request-ID",{exact:true})).toBeVisible();
});

test("OpenAPI is an authenticated full YAML download with no browser credential",async({page})=>{
  await mockOperatorConsole(page);
  await page.goto("/developer");
  const downloadLink=page.getByRole("link",{name:"Download OpenAPI YAML"});
  await expect(downloadLink).toHaveAttribute("href","/api/developer/openapi");
  await expect(downloadLink).toHaveAttribute("download","ledgersync-openapi.yaml");
  const downloadPromise=page.waitForEvent("download");
  await downloadLink.click();
  const download=await downloadPromise;
  expect(download.suggestedFilename()).toBe("ledgersync-openapi.yaml");
});

test("developer scope denial is distinct and suppresses metadata requests",async({page})=>{
  await mockOperatorConsole(page);
  let requested=false;
  await page.route("**/api/session",(route)=>json(route,{subject_id:"auditor",tenant_id:"tenant-1",csrf_token:"csrf",scopes:["events:read"],environment:"demo",tenant_label:"Meridian Labs · Test",operator_label:"Auditor"}));
  await page.route("**/api/developer/metadata",(route)=>{requested=true;return json(route,developerMetadata);});
  await page.goto("/developer");
  await expect(page.getByText("Developer contract not authorized")).toBeVisible();
  expect(requested).toBe(false);
  await expect(page.getByRole("button",{name:"Download OpenAPI YAML"})).toBeDisabled();
});

test("long code and contract tables reflow at 320px and 200-percent-equivalent width",async({page})=>{
  await mockOperatorConsole(page);
  for(const viewport of [{width:320,height:760},{width:640,height:900},{width:1440,height:900}]){
    await page.setViewportSize(viewport);
    await page.goto("/developer");
    await expect(page.locator("body")).toHaveJSProperty("scrollWidth",viewport.width);
    await expect(page.getByRole("heading",{name:"Developer",exact:true})).toBeVisible();
  }
  await page.emulateMedia({forcedColors:"active",reducedMotion:"reduce"});
  await page.goto("/developer");
  await expectAccessible(page);
});

test("offline state preserves loaded metadata and disables refresh and download",async({page,context})=>{
  await mockOperatorConsole(page);
  await page.goto("/developer");
  await expect(page.getByText(`v${developerMetadata.contract_version}`,{exact:true})).toBeVisible();
  await context.setOffline(true);
  await expect(page.getByText("Offline — contract freshness is not verified")).toBeVisible();
  await expect(page.getByRole("button",{name:"Refresh contract"})).toBeDisabled();
  await expect(page.getByRole("button",{name:"Download OpenAPI YAML"})).toBeDisabled();
  await context.setOffline(false);
});
