import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { NextRequest } from "next/server";

import { sanitizeLocalOrientation, sanitizeTransferExplainability } from "../../src/lib/api/orientation";
import { authorizeOperationsRead, isOperationsReadDenial, strictOperationsQuery } from "../../src/lib/operations-read";
import type { RateLimitStore } from "../../src/lib/rate-limit";
import type { Session } from "../../src/lib/session";

const origin="http://127.0.0.1:3000"; const allow:RateLimitStore={consume:async()=>({allowed:true,retryAfterSeconds:0})};
const session:Session={subjectId:"operator",tenantId:"tenant",csrfToken:"csrf",expiresAt:Date.now()+60_000,scopes:["explainability:read","transfers:read","events:read","reconciliation:read","local:read"]};
const request=(path="/api/local/orientation",method="GET",host="127.0.0.1:3000")=>new NextRequest(`${origin}${path}`,{method,headers:{host}});
const id=(digit:string)=>`${digit.repeat(8)}-${digit.repeat(4)}-4${digit.repeat(3)}-8${digit.repeat(3)}-${digit.repeat(12)}`;
const steps=[
  ["inspect_account","evidence_available","account_record"],["create_account","completed","account_created_audit"],["fund_account","completed","posted_transfer"],["inspect_transfer","evidence_available","transfer_record"],["run_reconciliation","completed","reconciliation_run"],["inspect_delivery","evidence_available","delivery_attempt"],["create_backup","completed","recovery_backup"],
].map(([step,state,evidence_type],index)=>({id:step,state,evidence_type,evidence_id:index===6?"backup-20260819T120000Z-abcdef1":id(String(index+1)),occurred_at:"2026-08-19T12:00:00Z"}));
const kinds=["request","transfer","journal_postings","balance_versions","outbox","delivery","reconciliation"];
const types=["idempotency_outcome","transfer","journal","balance_version","outbox_event","delivery_attempt","reconciliation_run"];
const stages=kinds.map((kind,index)=>({sequence:index+1,kind,state:"available",truncated:false,evidence:[{evidence_type:types[index],evidence_id:id(String(index+1)),occurred_at:`2026-08-19T12:00:0${index}Z`}]}));

test.beforeEach(()=>{process.env.LEDGERSYNC_PUBLIC_ORIGIN=origin;});

test("orientation and explainability reads require signed scopes, exact Host, GET, and no query",async()=>{
  assert.equal(isOperationsReadDenial(await authorizeOperationsRead(request(),session,"explainability:read",allow)),false);
  for(const candidate of [await authorizeOperationsRead(request(),{...session,scopes:[]},"explainability:read",allow),await authorizeOperationsRead(request(undefined,"POST"),session,"explainability:read",allow),await authorizeOperationsRead(request(undefined,"GET","attacker.invalid"),session,"explainability:read",allow)]) assert.ok(isOperationsReadDenial(candidate));
  assert.ok(strictOperationsQuery(request(),[]) instanceof URLSearchParams);
  assert.equal(strictOperationsQuery(request("/api/local/orientation?tenantId=attacker"),[]) instanceof URLSearchParams,false);
});

test("orientation sanitizer preserves seven durable states and rejects invented completion or hostile fields",()=>{
  const value={generated_at:"2026-08-19T12:05:00Z",evidence_state:"complete",steps};
  assert.equal(sanitizeLocalOrientation(200,value).status,200);
  assert.equal(sanitizeLocalOrientation(200,{...value,steps:[...steps.slice(0,6)]}).status,503);
  assert.equal(sanitizeLocalOrientation(200,{...value,steps:steps.map((step,index)=>index?step:{...step,state:"completed"})}).status,503);
  assert.equal(sanitizeLocalOrientation(200,{...value,steps:steps.map((step,index)=>index?step:{...step,filename:"backup.dump",token:"secret"})}).status,503);
  assert.equal(JSON.stringify(sanitizeLocalOrientation(503,{error:{code:"internal_error",message:"postgres://secret"}}).body).includes("secret"),false);
});

test("explainability sanitizer enforces semantic order, bounds, explicit gaps, and safe fields",()=>{
  const value={transfer_id:id("a"),generated_at:"2026-08-19T12:05:00Z",evidence_state:"complete",stages};
  assert.equal(sanitizeTransferExplainability(200,value).status,200);
  assert.equal(sanitizeTransferExplainability(200,{...value,stages:[stages[1],stages[0],...stages.slice(2)]}).status,503);
  assert.equal(sanitizeTransferExplainability(200,{...value,stages:stages.map((stage,index)=>index===4?{...stage,evidence:Array.from({length:9},()=>stage.evidence[0])}:stage)}).status,503);
  assert.equal(sanitizeTransferExplainability(200,{...value,stages:stages.map((stage,index)=>index===5?{...stage,state:"missing",evidence:[],reason_code:"no_delivery_attempts"}:stage),evidence_state:"partial"}).status,200);
  assert.equal(sanitizeTransferExplainability(200,{...value,stages:stages.map((stage,index)=>index===5?{...stage,endpoint:"https://private",raw_error:"token=secret"}:stage)}).status,503);
});

test("Phase 8 BFF surface is GET-only, fixed-path, encoded, and enforces all linked read scopes",async()=>{
  const root=process.cwd();
  const [orientation,explainability]=await Promise.all([readFile(`${root}/src/app/api/local/orientation/route.ts`,"utf8"),readFile(`${root}/src/app/api/transfers/[transferId]/explainability/route.ts`,"utf8")]);
  for(const source of [orientation,explainability]) { assert.match(source,/export async function GET/); assert.doesNotMatch(source,/export async function (POST|PATCH|DELETE)/); assert.match(source,/strictOperationsQuery\(request, \[\]\)/); }
  assert.match(orientation,/"\/api\/local\/orientation"/); assert.match(explainability,/encodeURIComponent\(transferId\)/); assert.match(explainability,/"transfers:read", "events:read", "reconciliation:read"/);
});
