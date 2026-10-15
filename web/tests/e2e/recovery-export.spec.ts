import AxeBuilder from "@axe-core/playwright";
import { expect,test,type Page,type Route } from "@playwright/test";

import { destinationAccount,diagnostics,mockOperatorConsole,recoveryEvidence,run } from "./fixtures";

test("Recovery labels an older schema as historical and recommends a current backup",async({page})=>{
  await mockOperatorConsole(page); await page.goto("/recovery");
  await expect(page.getByRole("heading",{name:"Recovery proof is historical",exact:true})).toBeVisible();
  await expect(page.getByText(/The backup schema differs from the current database/)).toBeVisible();
  await expect(page.getByRole("heading",{name:"Recovery evidence is complete",exact:true})).toHaveCount(0);
  await expect(page.getByRole("button",{name:"Copy Create protected backup command",exact:true})).toBeVisible();
});

test("Recovery requires a restore of the exact latest backup and limits snapshot claims",async({page})=>{
  await mockOperatorConsole(page);
  await page.route("**/api/local/diagnostics",route=>json(route,{...diagnostics,financial_authority:{...diagnostics.financial_authority,postgres:{state:"reachable",schema_version:recoveryEvidence.latest_backup.schema_version}}}));
  await page.route("**/api/recovery/manifests",route=>json(route,{...recoveryEvidence,latest_restore:{...recoveryEvidence.latest_restore,backup_id:"backup-20260818T115000Z-abcdef1"}}));
  await page.goto("/recovery");
  await expect(page.getByRole("heading",{name:"Latest backup needs a restore drill",exact:true})).toBeVisible();
  await page.route("**/api/recovery/manifests",route=>json(route,recoveryEvidence));
  await page.getByRole("button",{name:"Refresh evidence",exact:true}).click();
  await expect(page.getByRole("heading",{name:"Listed backup restore verified",exact:true})).toBeVisible();
  await expect(page.getByText(/does not prove coverage of later writes/)).toBeVisible();
  await page.route("**/api/recovery/manifests",route=>json(route,{error:{code:"temporary_unavailable"}},503));
  await page.getByRole("button",{name:"Refresh evidence",exact:true}).click();
  await expect(page.getByRole("heading",{name:"Recovery evidence is not current",exact:true})).toBeVisible();
});

function json(route:Route,body:unknown,status=200){return route.fulfill({status,contentType:"application/json",body:JSON.stringify(body)});}
test("candidate restore proof stays separate from normal backup coverage",async({page})=>{
  await mockOperatorConsole(page); await page.goto("/recovery");
  await expect(page.getByRole("heading",{name:"Recovery proof is historical",exact:true})).toBeVisible();
  await page.getByText("Recorded candidate restore",{exact:true}).click();
  await expect(page.getByRole("heading",{name:"Recorded candidate restore",exact:true})).toBeVisible();
  await expect(page.getByText(/does not prove that the normal database has a current backup/)).toBeVisible();
  await expect(page.getByText(/through the full named validation profile/)).toBeVisible();
  await expect(page.getByRole("button",{name:"Run restore",exact:true})).toHaveCount(0);
  await expect(page.getByRole("heading",{name:"Recovery proof is historical",exact:true})).toBeVisible();
});
async function expectAccessible(page:Page){const result=await new AxeBuilder({page}).exclude(".app-shell > aside").analyze();expect(result.violations.filter((item)=>["serious","critical"].includes(item.impact??""))).toEqual([]);}

test("Recovery separates current truth, backup, restore, retention, and copy-only host guidance",async({page,context})=>{
  await context.grantPermissions(["clipboard-read","clipboard-write"]); await mockOperatorConsole(page); await page.goto("/recovery");
  await expect(page.getByRole("heading",{name:"Recovery",exact:true})).toBeVisible();
  await expect(page.getByRole("heading",{name:"PostgreSQL now"})).toBeVisible();
  await expect(page.getByText("Digest verified",{exact:true})).toBeVisible();
  await expect(page.getByText("0 mismatches · normal environment unchanged",{exact:true})).toBeVisible();
  await expect(page.getByText("Restore and reset are intentionally absent",{exact:true})).toBeVisible();
  await expect(page.getByRole("button",{name:"Restore active database"})).toHaveCount(0);
  await page.getByText("Safe host commands", { exact:true }).click();
  await page.getByRole("button",{name:"Copy Run isolated restore drill command"}).click();
  expect(await page.evaluate(()=>navigator.clipboard.readText())).toBe(".\\scripts\\local-restore-drill.ps1");
  await expectAccessible(page);
});

test("Recovery shows truthful empty and partial evidence without inventing readiness",async({page})=>{
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
  await page.getByText("Filter history",{exact:true}).click(); await page.getByLabel("Search transfers").fill("11111111"); await page.getByText("Advanced filters",{exact:true}).click(); await page.getByLabel("Financial status").selectOption("posted"); await page.getByRole("button",{name:"Apply filters"}).click();
  await expect(page).toHaveURL(/q=11111111.*status=posted/);
  const trigger=page.getByRole("button",{name:"Export transfer details"}); await trigger.click();
  const dialog=page.getByRole("dialog",{name:"Review transfer history export"}); await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("heading",{name:"Review transfer history export"})).toBeFocused();
  await expect(dialog.getByText("Search: 11111111 · Financial status: posted",{exact:true})).toBeVisible();
  await expect(dialog.getByText("10,000",{exact:true})).toBeVisible(); await expect(dialog.getByText("This export is not a backup.",{exact:true})).toBeVisible();
  const downloadPromise=page.waitForEvent("download"); await dialog.getByRole("button",{name:"Download CSV"}).click(); const download=await downloadPromise;
  expect(download.suggestedFilename()).toBe("ledgersync-transfers-20260819T120000Z-v2.csv");
  await expect(page.getByRole("heading",{name:"Downloading exact CSV"})).toBeFocused();
  await expect(page.getByRole("button",{name:"Preparing export…"})).toBeDisabled();
});

test("account and reconciliation exports retain their exact contextual scope",async({page})=>{
  await mockOperatorConsole(page); await page.goto(`/accounts/${destinationAccount.account_id}`);
  await page.getByText("Transactions and export", { exact:true }).click();
  await page.getByRole("button",{name:"Export ledger history"}).click();
  await expect(page.getByRole("dialog",{name:"Review account ledger history export"}).getByText(`One authorized account · ${destinationAccount.account_id}`,{exact:true})).toBeVisible();
  await page.keyboard.press("Escape");
  await page.goto(`/reconciliation/${run.run_id}`); await page.getByRole("button",{name:"Export run result"}).click();
  const dialog=page.getByRole("dialog",{name:"Review reconciliation run export"});
  await expect(dialog.getByText(`One immutable run · ${run.run_id}`,{exact:true})).toBeVisible();
  await expect(dialog.getByText(`Run ID: ${run.run_id}`,{exact:true})).toBeVisible();
});

test("recovery and export controls reflow, deny missing scopes, and fail closed offline",async({page,context})=>{
  await mockOperatorConsole(page); await page.setViewportSize({width:320,height:760}); await page.goto("/recovery");
  await expect(page.locator("body")).toHaveJSProperty("scrollWidth",320); await expectAccessible(page);
  await page.route("**/api/session",route=>json(route,{subject_id:"reader",tenant_id:"tenant-1",csrf_token:"csrf",scopes:["local:read"],environment:"local"}));
  await page.goto("/recovery"); await expect(page.getByText("Recovery evidence not authorized",{exact:true})).toBeVisible();
  await expect(page.getByText("Recorded candidate restore",{exact:true})).toHaveCount(0);
  await mockOperatorConsole(page); await page.goto("/transfers");
  await expect(page.getByRole("button",{name:"Export transfer details"})).toBeVisible();
  await context.setOffline(true); await expect(page.getByRole("button",{name:"Export transfer details"})).toBeDisabled(); await context.setOffline(false);
});
