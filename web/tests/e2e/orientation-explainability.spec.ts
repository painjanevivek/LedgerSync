import AxeBuilder from "@axe-core/playwright";
import { expect,test,type Page,type Route } from "@playwright/test";

import { mockOperatorConsole,orientationEvidence,transfer,transferExplainability } from "./fixtures";
import type { LocalOrientation, OperatorPreferenceStepID } from "../../src/lib/api/orientation";

function json(route:Route,body:unknown,status=200){return route.fulfill({status,contentType:"application/json",body:JSON.stringify(body)});}
async function expectAccessible(page:Page){const result=await new AxeBuilder({page}).exclude(".app-shell > aside").analyze();expect(result.violations.filter((item)=>["serious","critical"].includes(item.impact??""))).toEqual([]);}

test("local guide persists confirmation, dismissal, and reopening in server-owned preferences",async({page,context})=>{
  await context.grantPermissions(["clipboard-read","clipboard-write"]); await mockOperatorConsole(page);
  let current=structuredClone(orientationEvidence) as LocalOrientation; const updates:unknown[]=[];
  await page.route("**/api/local/orientation",route=>json(route,current));
  await page.route("**/api/local/orientation/preferences",async route=>{
    const body=route.request().postDataJSON() as {expected_version:string;dismissed:boolean;completed_step_ids:string[]}; updates.push(body);
    current={...current,dismissed:body.dismissed,preference_version:String(Number(body.expected_version)+1),preference_updated_at:"2026-08-19T12:06:00Z",operator_completed_step_ids:body.completed_step_ids as OperatorPreferenceStepID[],steps:current.steps.map(step=>body.completed_step_ids.includes(step.id)?{...step,state:"operator_confirmed" as const,reason_code:undefined}:step)};
    return json(route,current);
  });
  await page.goto("/?guide=1");
  await expect(page.getByRole("heading",{name:"Follow one INR ledger record from system health to recovery"})).toBeVisible();
  await expect(page.getByText("PostgreSQL ledger",{exact:true})).toBeVisible();
  await expect(page.getByText("Ready to inspect",{exact:true}).first()).toBeVisible();
  const clearManualConfirmations=page.getByRole("button",{name:"Clear manual confirmations"});
  await expect(clearManualConfirmations).toBeDisabled();
  await expect(page.getByText("No manual confirmations are currently saved. Stored ledger evidence is never cleared here.",{exact:true})).toBeVisible();
  await expect(clearManualConfirmations).toHaveAttribute("aria-describedby",/.+/);
  await page.getByRole("button",{name:"I checked current health"}).first().click();
  await expect(page.getByText("Operator confirmed",{exact:true})).toBeVisible();
  await expect(clearManualConfirmations).toBeEnabled();
  await page.getByRole("button",{name:"Copy safe stop command"}).click();
  expect(await page.evaluate(()=>navigator.clipboard.readText())).toBe("powershell -File .\\scripts\\stop-local.ps1");
  await page.getByRole("button",{name:"Dismiss setup guide"}).click();
  await expect(page.getByRole("heading",{name:/Recommended next:/})).toBeVisible();
  await page.reload();
  await expect(page.getByRole("heading",{name:/Recommended next:/})).toBeVisible();
  await page.getByRole("button",{name:"Reopen setup guide"}).click();
  await expect(page.getByRole("heading",{name:"Follow one INR ledger record from system health to recovery"})).toBeVisible();
  expect(updates).toEqual([
    {expected_version:"0",dismissed:false,completed_step_ids:["confirm_health"]},
    {expected_version:"1",dismissed:true,completed_step_ids:["confirm_health"]},
    {expected_version:"2",dismissed:false,completed_step_ids:["confirm_health"]},
  ]);
  await expectAccessible(page);
});

test("unknown preference response refreshes server truth without optimistic completion",async({page})=>{
  await mockOperatorConsole(page);
  let reads=0; let writes=0;
  await page.route("**/api/local/orientation",route=>{reads+=1;return json(route,orientationEvidence);});
  await page.route("**/api/local/orientation/preferences",route=>{writes+=1;return json(route,{error:{code:"upstream_timeout"}},504);});
  await page.goto("/?guide=1");
  await page.getByRole("button",{name:"I checked current health"}).first().click();
  await expect(page.getByText(/The response was unknown, so current server state was refreshed without assuming the change succeeded/)).toBeVisible();
  await expect(page.getByText("Not yet evidenced",{exact:true}).first()).toBeVisible();
  expect(writes).toBe(1); expect(reads).toBeGreaterThanOrEqual(2);
});

test("orientation reports empty and unavailable durable evidence without browser-only completion claims",async({page})=>{
  await mockOperatorConsole(page);
  const partial={...orientationEvidence,evidence_state:"partial",steps:orientationEvidence.steps.map((step,index)=>index===2?{id:"inspect_accounts",state:"missing",evidence_type:"account_record",reason_code:"no_authorized_account"}:index===11?{id:"create_backup",state:"unavailable",evidence_type:"recovery_backup",reason_code:"recovery_evidence_unavailable"}:step)};
  await page.route("**/api/local/orientation",route=>json(route,partial)); await page.goto("/?guide=1");
  const orientation=page.locator(".local-orientation");
  await expect(orientation.getByText("Not yet evidenced",{exact:true})).toHaveCount(3);
  const recoveryStep=orientation.getByRole("listitem").filter({hasText:"Create and verify a backup"});
  await expect(recoveryStep.locator(".status-label")).toHaveText("Unavailable");
  const fundingStep=orientation.getByRole("listitem").filter({hasText:"Fund through an approved ledger event"});
  await expect(fundingStep.getByText("Stored evidence",{exact:true})).toBeVisible();
  await fundingStep.getByText("Fund through an approved ledger event",{exact:true}).click();
  await expect(fundingStep.getByRole("link",{name:"Open evidence"})).toHaveAttribute("href",`/funding/${orientationEvidence.steps[4].evidence_id}`);
  await expect(orientation.getByText("Stored evidence",{exact:true})).toHaveCount(4);
});

test("workspace and local guide fill the browser canvas on phone, tablet, and desktop",async({page})=>{
  await mockOperatorConsole(page);
  for (const viewport of [{width:390,height:844},{width:768,height:1024},{width:1440,height:900}]) {
    await page.setViewportSize(viewport);
    await page.goto("/?guide=1");
    await expect(page.getByRole("heading",{name:"Follow one INR ledger record from system health to recovery"})).toBeVisible();
    const geometry=await page.evaluate(()=>{
      const root=document.documentElement;
      const shell=document.querySelector<HTMLElement>(".app-shell")?.getBoundingClientRect();
      const main=document.querySelector<HTMLElement>(".console-main")?.getBoundingClientRect();
      if (!shell||!main) throw new Error("workspace geometry is unavailable");
      return {
        viewportWidth:window.innerWidth,
        viewportHeight:window.innerHeight,
        scrollWidth:root.scrollWidth,
        shellLeft:Math.round(shell.left),
        shellRight:Math.round(shell.right),
        shellHeight:Math.round(shell.height),
        mainRight:Math.round(main.right),
        undersizedText:[...document.querySelectorAll<HTMLElement>(".app-shell *")].filter((element)=>{
          if (!element.checkVisibility({visibilityProperty:true})) return false;
          const hasDirectText=[...element.childNodes].some((node)=>node.nodeType===Node.TEXT_NODE&&Boolean(node.textContent?.trim()));
          return hasDirectText&&Number.parseFloat(getComputedStyle(element).fontSize)<12;
        }).map((element)=>`${element.tagName.toLowerCase()}.${element.className}:${getComputedStyle(element).fontSize}`).slice(0,20),
      };
    });
    expect(geometry.scrollWidth).toBe(geometry.viewportWidth);
    expect(geometry.shellLeft).toBe(0);
    expect(geometry.shellRight).toBe(geometry.viewportWidth);
    expect(geometry.mainRight).toBe(geometry.viewportWidth);
    expect(geometry.shellHeight).toBeGreaterThanOrEqual(geometry.viewportHeight);
    expect(geometry.undersizedText).toEqual([]);
  }
});

test("transfer detail renders seven linked stored-evidence stages and preserves filtered return context",async({page})=>{
  await mockOperatorConsole(page); await page.goto(`/transfers?q=3333&status=posted`);
  await page.getByRole("link",{name:"Open record"}).click();
  await page.getByText("Evidence timeline and delivery",{exact:true}).click();
  await expect(page.getByRole("heading",{name:"Stored evidence chain"})).toBeVisible();
  await expect(page.getByRole("heading",{name:"Request and idempotency outcome"})).toBeVisible();
  await expect(page.getByRole("heading",{name:"Reconciliation coverage"})).toBeVisible();
  await expect(page.locator(".evidence-stage")).toHaveCount(7);
  await page.getByRole("link",{name:"← Back to previous view"}).click();
  await expect(page).toHaveURL(/\/transfers\?q=3333&status=posted/);
  await expect(page.getByLabel("Search transfers")).toHaveValue("3333");
  await expect(page.getByLabel("Status")).toHaveValue("posted");
});

test("partial, out-of-order, denied, and compact timeline states remain explicit and accessible",async({page})=>{
  await mockOperatorConsole(page); await page.setViewportSize({width:320,height:760});
  const partial={...transferExplainability,evidence_state:"partial",stages:transferExplainability.stages.map((stage,index)=>index===5?{...stage,state:"missing",evidence:[],reason_code:"no_delivery_attempts"}:index===4?{...stage,evidence:stage.evidence.map((item)=>({...item,occurred_at:"2026-08-19T10:00:00Z"}))}:stage)};
  await page.route("**/api/transfers/*/explainability",route=>json(route,partial)); await page.goto(`/transfers/${transfer.transfer_id}`);
  await page.getByText("Evidence timeline and delivery",{exact:true}).click();
  await expect(page.getByText("Stored timestamps are out of sequence",{exact:true})).toBeVisible();
  await expect(page.getByText("No downstream delivery attempt is stored.",{exact:true})).toBeVisible();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth>document.documentElement.clientWidth)).toBe(false);
  await expectAccessible(page);
  await page.route("**/api/session",route=>json(route,{subject_id:"reader",tenant_id:"tenant-1",csrf_token:"csrf",scopes:["transfers:read"],environment:"local"})); await page.reload();
  await page.getByText("Evidence timeline and delivery",{exact:true}).click();
  await expect(page.getByText("Stored evidence timeline not authorized",{exact:true})).toBeVisible();
});
