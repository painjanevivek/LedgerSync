import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { NextRequest } from "next/server";

import { parseOrientationPreferenceInput, sanitizeLocalOrientation, sanitizeTransferExplainability } from "../../src/lib/api/orientation";
import { authorizeOperationsRead, isOperationsReadDenial, strictOperationsQuery } from "../../src/lib/operations-read";
import type { RateLimitStore } from "../../src/lib/rate-limit";
import type { Session } from "../../src/lib/session";

const origin="http://127.0.0.1:3000"; const allow:RateLimitStore={consume:async()=>({allowed:true,retryAfterSeconds:0})};
const session:Session={subjectId:"operator",tenantId:"tenant",csrfToken:"csrf",expiresAt:Date.now()+60_000,scopes:["explainability:read","transfers:read","events:read","reconciliation:read","local:read","local:write"]};
const request=(path="/api/local/orientation",method="GET",host="127.0.0.1:3000")=>new NextRequest(`${origin}${path}`,{method,headers:{host}});
const id=(digit:string)=>`${digit.repeat(8)}-${digit.repeat(4)}-4${digit.repeat(3)}-8${digit.repeat(3)}-${digit.repeat(12)}`;
const steps=[
  {id:"confirm_health",state:"operator_confirmed",evidence_type:"local_health_confirmation"},
  {id:"understand_authority",state:"missing",evidence_type:"authority_acknowledgement",reason_code:"operator_confirmation_required"},
  {id:"inspect_accounts",state:"evidence_available",evidence_type:"account_record",evidence_id:id("1"),occurred_at:"2026-08-19T12:00:00Z",reason_code:"operator_confirmation_required"},
  {id:"create_account",state:"completed",evidence_type:"account_created_audit",evidence_id:id("2"),occurred_at:"2026-08-19T12:00:00Z"},
  {id:"fund_account",state:"completed",evidence_type:"funding_journal",evidence_id:id("6"),occurred_at:"2026-08-19T12:00:00Z"},
  {id:"post_transfer",state:"completed",evidence_type:"posted_transfer",evidence_id:id("3"),occurred_at:"2026-08-19T12:00:00Z"},
  {id:"retry_transfer",state:"evidence_available",evidence_type:"idempotency_outcome",evidence_id:id("3"),occurred_at:"2026-08-19T12:00:00Z",reason_code:"operator_confirmation_required"},
  {id:"inspect_postings",state:"evidence_available",evidence_type:"journal_postings",evidence_id:id("3"),occurred_at:"2026-08-19T12:00:00Z",reason_code:"operator_confirmation_required"},
  {id:"run_reconciliation",state:"completed",evidence_type:"reconciliation_run",evidence_id:id("4"),occurred_at:"2026-08-19T12:00:00Z"},
  {id:"inspect_delivery",state:"evidence_available",evidence_type:"delivery_attempt",evidence_id:id("5"),occurred_at:"2026-08-19T12:00:00Z",reason_code:"operator_confirmation_required"},
  {id:"export_evidence",state:"evidence_available",evidence_type:"evidence_export",evidence_id:id("3"),occurred_at:"2026-08-19T12:00:00Z",reason_code:"operator_confirmation_required"},
  {id:"create_backup",state:"completed",evidence_type:"recovery_backup",evidence_id:"backup-20260819T120000Z-abcdef1",occurred_at:"2026-08-19T12:00:00Z"},
];
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

test("orientation sanitizer preserves twelve evidence and preference states without inventing financial completion",()=>{
  const value={generated_at:"2026-08-19T12:05:00Z",evidence_state:"partial",dismissed:false,preference_version:"2",preference_updated_at:"2026-08-19T12:01:00Z",operator_completed_step_ids:["confirm_health"],steps};
  assert.equal(sanitizeLocalOrientation(200,value).status,200);
  assert.equal(sanitizeLocalOrientation(200,{...value,steps:[...steps.slice(0,11)]}).status,503);
  assert.equal(sanitizeLocalOrientation(200,{...value,steps:steps.map((step,index)=>index?step:{...step,state:"completed"})}).status,503);
  assert.equal(sanitizeLocalOrientation(200,{...value,operator_completed_step_ids:[]}).status,503);
  assert.equal(sanitizeLocalOrientation(200,{...value,steps:steps.map((step,index)=>index?step:{...step,filename:"backup.dump",token:"secret"})}).status,503);
  assert.equal(JSON.stringify(sanitizeLocalOrientation(503,{error:{code:"internal_error",message:"postgres://secret"}}).body).includes("secret"),false);
});

test("orientation preference input is exact, bounded, sorted, and rejects financial-step completion",()=>{
  assert.deepEqual(parseOrientationPreferenceInput({expected_version:"2",dismissed:true,completed_step_ids:["retry_transfer","confirm_health"]}),{expected_version:"2",dismissed:true,completed_step_ids:["confirm_health","retry_transfer"]});
  for(const value of [{expected_version:"2",dismissed:false,completed_step_ids:["fund_account"]},{expected_version:"-1",dismissed:false,completed_step_ids:[]},{expected_version:"2",dismissed:false,completed_step_ids:[],token:"secret"}]) assert.throws(()=>parseOrientationPreferenceInput(value));
});

test("explainability sanitizer enforces semantic order, bounds, explicit gaps, and safe fields",()=>{
  const value={transfer_id:id("a"),generated_at:"2026-08-19T12:05:00Z",evidence_state:"complete",stages};
  assert.equal(sanitizeTransferExplainability(200,value).status,200);
  assert.equal(sanitizeTransferExplainability(200,{...value,stages:[stages[1],stages[0],...stages.slice(2)]}).status,503);
  assert.equal(sanitizeTransferExplainability(200,{...value,stages:stages.map((stage,index)=>index===4?{...stage,evidence:Array.from({length:9},()=>stage.evidence[0])}:stage)}).status,503);
  assert.equal(sanitizeTransferExplainability(200,{...value,stages:stages.map((stage,index)=>index===5?{...stage,state:"missing",evidence:[],reason_code:"no_delivery_attempts"}:stage),evidence_state:"partial"}).status,200);
  assert.equal(sanitizeTransferExplainability(200,{...value,stages:stages.map((stage,index)=>index===5?{...stage,endpoint:"https://private",raw_error:"token=secret"}:stage)}).status,503);
});

test("guidance BFF keeps reads fixed and protects server-owned preference writes",async()=>{
  const root=process.cwd();
  const [orientation,preference,explainability]=await Promise.all([readFile(`${root}/src/app/api/local/orientation/route.ts`,"utf8"),readFile(`${root}/src/app/api/local/orientation/preferences/route.ts`,"utf8"),readFile(`${root}/src/app/api/transfers/[transferId]/explainability/route.ts`,"utf8")]);
  for(const source of [orientation,explainability]) { assert.match(source,/export async function GET/); assert.doesNotMatch(source,/export async function (POST|PATCH|DELETE)/); assert.match(source,/strictOperationsQuery\(request, \[\]\)/); }
  assert.match(orientation,/"\/api\/local\/orientation"/); assert.match(explainability,/encodeURIComponent\(transferId\)/); assert.match(explainability,/"transfers:read", "events:read", "reconciliation:read"/);
  assert.match(preference,/export async function PUT/); assert.match(preference,/"local:write"/); assert.match(preference,/hasValidCSRF/); assert.match(preference,/parseOrientationPreferenceInput/); assert.match(preference,/searchParams\.size/); assert.doesNotMatch(preference,/localStorage|cookie.*dismiss/i);
});
