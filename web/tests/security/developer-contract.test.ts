import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { NextRequest } from "next/server";

import { sanitizeDeveloperMetadata } from "../../src/lib/api/developer";
import { isSafeOpenAPIYAML } from "../../src/lib/developer-read";
import { authorizeOperationsRead, isOperationsReadDenial, strictOperationsQuery } from "../../src/lib/operations-read";
import type { RateLimitStore } from "../../src/lib/rate-limit";
import type { Session } from "../../src/lib/session";

const origin="http://127.0.0.1:3000";
const session:Session={subjectId:"developer-1",tenantId:"tenant-1",csrfToken:"csrf",expiresAt:Date.now()+60_000,scopes:["developer:read"]};
const allow:RateLimitStore={consume:async()=>({allowed:true,retryAfterSeconds:0})};
function request(path="/api/developer/metadata",method="GET",host="127.0.0.1:3000"){return new NextRequest(`${origin}${path}`,{method,headers:{host}});}

test.beforeEach(()=>{process.env.LEDGERSYNC_PUBLIC_ORIGIN=origin;});

test("developer routes require a signed developer scope, exact Host, GET, and bounded rate",async()=>{
  assert.equal(isOperationsReadDenial(await authorizeOperationsRead(request(),session,"developer:read",allow)),false);
  const denials=[await authorizeOperationsRead(request(),null,"developer:read",allow),await authorizeOperationsRead(request(),{...session,scopes:["events:read"]},"developer:read",allow),await authorizeOperationsRead(request(undefined,"GET","attacker.example:3000"),session,"developer:read",allow),await authorizeOperationsRead(request(undefined,"POST"),session,"developer:read",allow)];
  assert.deepEqual(denials.map((value)=>isOperationsReadDenial(value)?value.status:0),[401,403,400,405]);
  assert.equal(strictOperationsQuery(request("/api/developer/metadata?url=http://attacker.example"),[]) instanceof URLSearchParams,false);
});

test("the UI consumes the canonical versioned metadata and exact-string examples",async()=>{
  const raw=await readFile("../contracts/developer-examples.v1.json","utf8");
  const canonical=JSON.parse(raw) as Record<string,unknown>;
  const result=sanitizeDeveloperMetadata(200,canonical);
  assert.equal(result.status,200);
  const examples=result.body.examples as Array<{id:string;body:Record<string,string>;result_facts?:Record<string,string>;headers:Record<string,string>}>;
  const transfer=examples.find((example)=>example.id==="create_transfer");
  const account=examples.find((example)=>example.id==="create_account");
  assert.equal(transfer?.body.amount,"125.50");
  assert.equal(typeof transfer?.body.amount,"string");
  assert.equal(account?.result_facts?.available_minor,"0");
  assert.equal(account?.result_facts?.ledger_minor,"0");
  assert.ok(examples.every((example)=>!/authorization|token|secret/i.test(JSON.stringify(example.headers))));

  const openapi=await readFile("../contracts/openapi.yaml","utf8");
  for(const example of examples){
    assert.match(openapi,new RegExp(`operationId: ${example.id==="create_transfer"?"createTransfer":"createAccount"}`));
  }
  assert.match(openapi,/amount: \{ type: string,/);
  assert.match(openapi,/available_minor: \{ \$ref: '#\/components\/schemas\/ExactMinor' \}/);
});

test("metadata sanitizer rejects arbitrary URLs, credentials, numeric money, drift, and unknown fields",async()=>{
  const canonical=JSON.parse(await readFile("../contracts/developer-examples.v1.json","utf8")) as Record<string,unknown>;
  const examples=canonical.examples as Array<Record<string,unknown>>;
  const transfer=examples[0];
  const hostile=[
    {...canonical,arbitrary_url:"http://attacker.example"},
    {...canonical,base_url:"http://api:8080/api"},
    {...canonical,examples:[{...transfer,headers:{...(transfer.headers as object),Authorization:"Bearer secret"}},examples[1]]},
    {...canonical,examples:[{...transfer,body:{...(transfer.body as object),amount:125.5}},examples[1]]},
    {...canonical,endpoint_groups:[...(canonical.endpoint_groups as unknown[]),{id:"runner",label:"Runner",operations:[]}]},
  ];
  for(const value of hostile)assert.equal(sanitizeDeveloperMetadata(200,value).status,503);
});

test("OpenAPI download validation accepts the canonical contract and rejects secret-bearing or malformed YAML",async()=>{
  const canonical=await readFile("../contracts/openapi.yaml","utf8");
  assert.equal(isSafeOpenAPIYAML(canonical),true);
  assert.equal(isSafeOpenAPIYAML("openapi: 3.1.0\ninfo:\npaths:\ncomponents:\nLEDGERSYNC_PRIVATE_API_TOKEN=secret"),false);
  assert.equal(isSafeOpenAPIYAML("openapi: 3.1.0\ninfo:\npaths:\ncomponents:\nAuthorization: Bearer abcdefghijklmnopqrstuvwxyz"),false);
  assert.equal(isSafeOpenAPIYAML("not yaml"),false);
});

test("developer browser boundary has no arbitrary request runner or raw credential endpoint",async()=>{
  const sources=await Promise.all(["src/app/api/developer/metadata/route.ts","src/app/api/developer/openapi/route.ts","src/lib/developer-read.ts","src/features/developer/DeveloperViews.tsx"].map((path)=>readFile(path,"utf8")));
  assert.ok(sources.slice(0,2).every((source)=>source.includes("export async function GET")&&!source.includes("export async function POST")));
  assert.match(sources[2],/\/api\/openapi\.yaml/);
  assert.doesNotMatch(sources.join("\n"),/api\/developer\/(?:request|runner|credential|token|reveal)/i);
  assert.doesNotMatch(sources[3],/name=["'](?:url|authorization|token|header)["']/i);
});
