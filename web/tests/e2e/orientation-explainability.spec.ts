import AxeBuilder from "@axe-core/playwright";
import { expect,test,type Page,type Route } from "@playwright/test";

import { mockOperatorConsole,orientationEvidence,transfer,transferExplainability } from "./fixtures";

function json(route:Route,body:unknown,status=200){return route.fulfill({status,contentType:"application/json",body:JSON.stringify(body)});}
async function expectAccessible(page:Page){const result=await new AxeBuilder({page}).exclude(".app-shell > aside").analyze();expect(result.violations.filter((item)=>["serious","critical"].includes(item.impact??""))).toEqual([]);}

test("local guide reads durable evidence, dismisses without blocking, and reopens from Local tools",async({page,context})=>{
  await context.grantPermissions(["clipboard-read","clipboard-write"]); await mockOperatorConsole(page); await page.goto("/");
  await expect(page.getByRole("heading",{name:"Follow one INR ledger record from intent to evidence"})).toBeVisible();
  await expect(page.getByText("PostgreSQL ledger",{exact:true})).toBeVisible();
  await expect(page.getByText("Evidence available",{exact:true}).first()).toBeVisible();
  await page.getByRole("button",{name:"Copy safe stop command"}).click();
  expect(await page.evaluate(()=>navigator.clipboard.readText())).toBe("powershell -File .\\scripts\\stop-local.ps1");
  await page.getByRole("button",{name:"Dismiss local guide"}).click();
  await expect(page.getByRole("heading",{name:"Follow one INR ledger record from intent to evidence"})).toHaveCount(0);
  await page.getByRole("link",{name:"Local guide"}).click();
  await expect(page).toHaveURL(/\?guide=1/);
  await expect(page.getByRole("heading",{name:"Follow one INR ledger record from intent to evidence"})).toBeVisible();
  await expectAccessible(page);
});

test("orientation reports empty and unavailable durable evidence without browser-only completion claims",async({page})=>{
  await mockOperatorConsole(page);
  const partial={...orientationEvidence,evidence_state:"partial",steps:orientationEvidence.steps.map((step,index)=>index===0?{id:"inspect_account",state:"missing",evidence_type:"account_record",reason_code:"no_authorized_account"}:index===6?{id:"create_backup",state:"unavailable",evidence_type:"recovery_backup",reason_code:"recovery_evidence_unavailable"}:step)};
  await page.route("**/api/local/orientation",route=>json(route,partial)); await page.goto("/?guide=1");
  await expect(page.getByText("Not yet evidenced",{exact:true})).toBeVisible();
  await expect(page.getByText("Evidence unavailable",{exact:true})).toBeVisible();
  await expect(page.getByText("Completed",{exact:true})).toHaveCount(3);
});

test("workspace and local guide fill the browser canvas on phone, tablet, and desktop",async({page})=>{
  await mockOperatorConsole(page);
  for (const viewport of [{width:390,height:844},{width:768,height:1024},{width:1440,height:900}]) {
    await page.setViewportSize(viewport);
    await page.goto("/?guide=1");
    await expect(page.getByRole("heading",{name:"Follow one INR ledger record from intent to evidence"})).toBeVisible();
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
      };
    });
    expect(geometry.scrollWidth).toBe(geometry.viewportWidth);
    expect(geometry.shellLeft).toBe(0);
    expect(geometry.shellRight).toBe(geometry.viewportWidth);
    expect(geometry.mainRight).toBe(geometry.viewportWidth);
    expect(geometry.shellHeight).toBeGreaterThanOrEqual(geometry.viewportHeight);
  }
});

test("transfer detail renders seven linked stored-evidence stages and preserves filtered return context",async({page})=>{
  await mockOperatorConsole(page); await page.goto(`/transfers?q=3333&status=posted`);
  await page.getByRole("link",{name:"Open record"}).click();
  await expect(page.getByRole("heading",{name:"Stored evidence chain"})).toBeVisible();
  await expect(page.getByRole("heading",{name:"Request and idempotency outcome"})).toBeVisible();
  await expect(page.getByRole("heading",{name:"Reconciliation coverage"})).toBeVisible();
  await expect(page.locator(".evidence-stage")).toHaveCount(7);
  await page.getByRole("link",{name:"← Back to previous view"}).click();
  await expect(page).toHaveURL(/\/transfers\?q=3333&status=posted/);
  await expect(page.getByLabel("Search transfers")).toHaveValue("3333");
  await expect(page.getByLabel("Financial status")).toHaveValue("posted");
});

test("partial, out-of-order, denied, and compact timeline states remain explicit and accessible",async({page})=>{
  await mockOperatorConsole(page); await page.setViewportSize({width:320,height:760});
  const partial={...transferExplainability,evidence_state:"partial",stages:transferExplainability.stages.map((stage,index)=>index===5?{...stage,state:"missing",evidence:[],reason_code:"no_delivery_attempts"}:index===4?{...stage,evidence:stage.evidence.map((item)=>({...item,occurred_at:"2026-08-19T10:00:00Z"}))}:stage)};
  await page.route("**/api/transfers/*/explainability",route=>json(route,partial)); await page.goto(`/transfers/${transfer.transfer_id}`);
  await expect(page.getByText("Stored timestamps are out of sequence",{exact:true})).toBeVisible();
  await expect(page.getByText("No downstream delivery attempt is stored.",{exact:true})).toBeVisible();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth>document.documentElement.clientWidth)).toBe(false);
  await expectAccessible(page);
  await page.route("**/api/session",route=>json(route,{subject_id:"reader",tenant_id:"tenant-1",csrf_token:"csrf",scopes:["transfers:read"],environment:"demo"})); await page.reload();
  await expect(page.getByText("Stored evidence timeline not authorized",{exact:true})).toBeVisible();
});
