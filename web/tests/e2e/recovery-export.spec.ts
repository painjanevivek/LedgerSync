import AxeBuilder from "@axe-core/playwright";
import { expect,test,type Page,type Route } from "@playwright/test";

import { destinationAccount,mockOperatorConsole,recoveryEvidence,run } from "./fixtures";

function json(route:Route,body:unknown,status=200){return route.fulfill({status,contentType:"application/json",body:JSON.stringify(body)});}
async function expectAccessible(page:Page){const result=await new AxeBuilder({page}).exclude(".app-shell > aside").analyze();expect(result.violations.filter((item)=>["serious","critical"].includes(item.impact??""))).toEqual([]);}

test("Recovery Center separates current truth, backup, restore, retention, and copy-only host guidance",async({page,context})=>{
  await context.grantPermissions(["clipboard-read","clipboard-write"]); await mockOperatorConsole(page); await page.goto("/recovery");
  await expect(page.getByRole("heading",{name:"Recovery Center"})).toBeVisible();
  await expect(page.getByRole("heading",{name:"PostgreSQL now"})).toBeVisible();
  await expect(page.getByText("Digest verified",{exact:true})).toBeVisible();
  await expect(page.getByText("0 mismatches · normal environment unchanged",{exact:true})).toBeVisible();
  await expect(page.getByText("Restore and reset are intentionally absent",{exact:true})).toBeVisible();
  await expect(page.getByRole("button",{name:"Restore active database"})).toHaveCount(0);
  await page.getByRole("button",{name:"Copy Run isolated restore drill command"}).click();
  expect(await page.evaluate(()=>navigator.clipboard.readText())).toBe(".\\scripts\\local-restore-drill.ps1");
  await expectAccessible(page);
});

test("Recovery Center shows truthful empty and partial evidence without inventing readiness",async({page})=>{
  await mockOperatorConsole(page);
  await page.route("**/api/recovery/manifests",route=>json(route,{...recoveryEvidence,latest_backup:null,latest_restore:null,retention:{valid_backup_count:0,ignored_entry_count:1,configured_keep_count:5}}));
  await page.route("**/api/local/diagnostics",route=>json(route,{error:{code:"temporary_unavailable"}},503));
  await page.goto("/recovery");
  await expect(page.getByText("Current database evidence unavailable",{exact:true})).toBeVisible();
  await expect(page.getByText("No finalized backup evidence",{exact:true})).toBeVisible();
  await expect(page.getByText("No isolated restore evidence",{exact:true})).toBeVisible();
  await expect(page.getByText("Some recovery entries were excluded",{exact:true})).toBeVisible();
  await expect(page.getByText("Digest verified",{exact:true})).toHaveCount(0);
});

test("transfer export review discloses exact filters and uses one native bounded download",async({page})=>{
  await mockOperatorConsole(page); await page.goto("/transfers");
  await page.getByLabel("Search transfers").fill("11111111"); await page.getByLabel("Financial status").selectOption("posted");
  const trigger=page.getByRole("button",{name:"Export transfer evidence"}); await trigger.click();
  const dialog=page.getByRole("dialog",{name:"Review transfer history export"}); await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("heading",{name:"Review transfer history export"})).toBeFocused();
  await expect(dialog.getByText("Search: 11111111 · Financial status: posted",{exact:true})).toBeVisible();
  await expect(dialog.getByText("10,000",{exact:true})).toBeVisible(); await expect(dialog.getByText("This export is not a backup.",{exact:true})).toBeVisible();
  const downloadPromise=page.waitForEvent("download"); await dialog.getByRole("button",{name:"Download CSV"}).click(); const download=await downloadPromise;
  expect(download.suggestedFilename()).toBe("ledgersync-transfers-20260819T120000Z-v1.csv");
  await expect(page.getByRole("heading",{name:"Downloading exact CSV"})).toBeFocused();
  await expect(page.getByRole("button",{name:"Preparing export…"})).toBeDisabled();
});

test("account and reconciliation exports retain their exact contextual scope",async({page})=>{
  await mockOperatorConsole(page); await page.goto(`/accounts/${destinationAccount.account_id}`);
  await page.getByRole("button",{name:"Export ledger evidence"}).click();
  await expect(page.getByRole("dialog",{name:"Review account ledger history export"}).getByText(`One authorized account · ${destinationAccount.account_id}`,{exact:true})).toBeVisible();
  await page.keyboard.press("Escape");
  await page.goto(`/reconciliation/${run.run_id}`); await page.getByRole("button",{name:"Export run evidence"}).click();
  const dialog=page.getByRole("dialog",{name:"Review reconciliation run export"});
  await expect(dialog.getByText(`One immutable run · ${run.run_id}`,{exact:true})).toBeVisible();
  await expect(dialog.getByText(`Run ID: ${run.run_id}`,{exact:true})).toBeVisible();
});

test("recovery and export controls reflow, deny missing scopes, and fail closed offline",async({page,context})=>{
  await mockOperatorConsole(page); await page.setViewportSize({width:320,height:760}); await page.goto("/recovery");
  await expect(page.locator("body")).toHaveJSProperty("scrollWidth",320); await expectAccessible(page);
  await page.route("**/api/session",route=>json(route,{subject_id:"reader",tenant_id:"tenant-1",csrf_token:"csrf",scopes:["local:read"],environment:"demo"}));
  await page.goto("/recovery"); await expect(page.getByText("Recovery evidence not authorized",{exact:true})).toBeVisible();
  await mockOperatorConsole(page); await page.goto("/transfers"); await context.setOffline(true);
  await expect(page.getByRole("button",{name:"Export transfer evidence"})).toBeDisabled(); await context.setOffline(false);
});
