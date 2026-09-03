import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { NextRequest } from "next/server";

import { sanitizeRecoveryIndex } from "../../src/lib/api/recovery";
import { authorizeEvidenceExport, isEvidenceExportDenial, sanitizeExportHeaders, strictExportQuery } from "../../src/lib/export-read";
import { authorizeOperationsRead, isOperationsReadDenial, strictOperationsQuery } from "../../src/lib/operations-read";
import type { RateLimitStore } from "../../src/lib/rate-limit";
import type { Session } from "../../src/lib/session";

const origin="http://127.0.0.1:3000";
const session:Session={subjectId:"operator-1",tenantId:"tenant-1",csrfToken:"csrf",expiresAt:Date.now()+60_000,roles:["tenant:operator"],scopes:["recovery:read","exports:read","transfers:read","transactions:read","reconciliation:read"]};
const allow:RateLimitStore={consume:async()=>({allowed:true,retryAfterSeconds:0})};
const recovery={format_version:"ledgersync-recovery-evidence-index/v1",generated_at_utc:"2026-08-25T10:00:00Z",latest_backup:{backup_id:"backup-20260825T090000Z-abcdef1",finalized_at_utc:"2026-08-25T09:00:00Z",size_bytes:1048576,schema_version:"000008_account_commands",digest_status:"verified",validation_status:"passed",source_commit:"0123456789abcdef0123456789abcdef01234567"},latest_restore:{backup_id:"backup-20260825T090000Z-abcdef1",completed_at_utc:"2026-08-25T09:15:00Z",status:"passed",reconciliation_status:"matched",mismatch_count:0,normal_project_unchanged:true,local_rto_seconds:42.5},retention:{valid_backup_count:3,ignored_entry_count:0,configured_keep_count:5}};
function request(path="/api/recovery/manifests",method="GET",host="127.0.0.1:3000"){return new NextRequest(`${origin}${path}`,{method,headers:{host}});}

test.beforeEach(()=>{process.env.LEDGERSYNC_PUBLIC_ORIGIN=origin;});

test("recovery reads require a signed scoped session, exact Host, GET, and no caller query",async()=>{
  assert.equal(isOperationsReadDenial(await authorizeOperationsRead(request(),session,"recovery:read",allow)),false);
  const denied=[await authorizeOperationsRead(request(),null,"recovery:read",allow),await authorizeOperationsRead(request(),{...session,scopes:["local:read"]},"recovery:read",allow),await authorizeOperationsRead(request(undefined,"GET","attacker.example:3000"),session,"recovery:read",allow),await authorizeOperationsRead(request(undefined,"POST"),session,"recovery:read",allow)];
  assert.deepEqual(denied.map((item)=>isOperationsReadDenial(item)?item.status:0),[401,403,400,405]);
  assert.ok(strictOperationsQuery(request(),[]) instanceof URLSearchParams);
  assert.equal(strictOperationsQuery(request("/api/recovery/manifests?path=C:%5Cbackup"),[]) instanceof URLSearchParams,false);
});

test("recovery sanitizer preserves only the fixed verified custody index",()=>{
  assert.equal(sanitizeRecoveryIndex(200,recovery).status,200);
  assert.equal(sanitizeRecoveryIndex(200,{...recovery,latest_backup:null,latest_restore:null,retention:{valid_backup_count:0,ignored_entry_count:0,configured_keep_count:5}}).status,200);
  const hostile=[{...recovery,path:"C:\\backups"},{...recovery,latest_backup:{...recovery.latest_backup,filename:"dump.sql"}},{...recovery,latest_backup:{...recovery.latest_backup,digest:"abcdef"}},{...recovery,latest_restore:{...recovery.latest_restore,mismatch_count:1}},{...recovery,latest_restore:{...recovery.latest_restore,normal_project_unchanged:false}},{...recovery,latest_backup:null},{...recovery,retention:{...recovery.retention,configured_keep_count:0}}];
  for(const value of hostile)assert.equal(sanitizeRecoveryIndex(200,value).status,503);
});

test("exports require exports:read plus the underlying read scope and exact Host",async()=>{
  assert.equal(isEvidenceExportDenial(await authorizeEvidenceExport(request("/api/exports/transfers.csv"),session,"transfers:read")),false);
  const denied=[await authorizeEvidenceExport(request("/api/exports/transfers.csv"),null,"transfers:read"),await authorizeEvidenceExport(request("/api/exports/transfers.csv"),{...session,scopes:["transfers:read"]},"transfers:read"),await authorizeEvidenceExport(request("/api/exports/transfers.csv"),{...session,scopes:["exports:read"]},"transfers:read"),await authorizeEvidenceExport(request("/api/exports/transfers.csv","GET","attacker.example:3000"),session,"transfers:read"),await authorizeEvidenceExport(request("/api/exports/transfers.csv","POST"),session,"transfers:read")];
  assert.deepEqual(denied.map((item)=>isEvidenceExportDenial(item)?item.status:0),[401,403,403,400,405]);
});

test("export queries are strict, bounded, typed, and cannot select tenant, filename, or path",()=>{
  const valid=strictExportQuery(request("/api/exports/transfers.csv?status=pending&q=ABC-1&from=2026-08-01T00:00:00Z&to=2026-08-25T00:00:00Z&limit=10000"),"transfers");
  assert.ok(valid instanceof URLSearchParams);
  for(const path of ["/api/exports/transfers.csv?tenantId=tenant-2","/api/exports/transfers.csv?filename=report.csv","/api/exports/transfers.csv?path=C:%5Cbackup","/api/exports/transfers.csv?status=posted&status=rejected","/api/exports/transfers.csv?limit=10001","/api/exports/transfers.csv?status=unknown","/api/exports/transfers.csv?accountId=secret","/api/exports/transfers.csv?from=2026-08-26T00:00:00Z&to=2026-08-25T00:00:00Z","/api/exports/transfers.csv?from=2026-08-01T00:00:00"]){assert.equal(strictExportQuery(request(path),"transfers") instanceof URLSearchParams,false,path);}
  assert.ok(strictExportQuery(request("/api/exports/reconciliation.csv?runId=55555555-5555-4555-8555-555555555555&status=matched&limit=1"),"reconciliation") instanceof URLSearchParams);
});

test("CSV headers retain only a canonical filename, schema, media type, and safe correlation",()=>{
  const canonical=new Headers({"content-type":"text/csv; charset=utf-8","content-disposition":"attachment; filename=\"ledgersync-transfers-20260825T101112Z-v2.csv\"","x-ledgersync-export-schema":"2","x-request-id":"request-1","authorization":"Bearer secret","x-internal-path":"C:\\backup"});
  assert.deepEqual(sanitizeExportHeaders(canonical,"transfers"),{"Cache-Control":"no-store","Content-Type":"text/csv; charset=utf-8","Content-Disposition":"attachment; filename=\"ledgersync-transfers-20260825T101112Z-v2.csv\"","X-LedgerSync-Export-Schema":"2","X-Content-Type-Options":"nosniff","X-Request-ID":"request-1"});
  for(const headers of [new Headers({...Object.fromEntries(canonical),"content-disposition":"attachment; filename=\"../../secret.csv\""}),new Headers({...Object.fromEntries(canonical),"content-disposition":"attachment; filename=\"ledgersync-reconciliation-20260825T101112Z-v2.csv\""}),new Headers({...Object.fromEntries(canonical),"x-ledgersync-export-schema":"1"}),new Headers({...Object.fromEntries(canonical),"content-type":"application/octet-stream"})])assert.equal(sanitizeExportHeaders(headers,"transfers"),null);
});

test("web recovery and exports remain GET-only, fixed-route, streamed, and non-executing",async()=>{
  const files=["src/app/api/recovery/manifests/route.ts","src/app/api/exports/transfers.csv/route.ts","src/app/api/exports/accounts/[accountId]/transactions.csv/route.ts","src/app/api/exports/reconciliation.csv/route.ts","src/lib/export-read.ts","src/features/recovery/RecoveryView.tsx","src/features/exports/EvidenceExportControl.tsx"];
  const sources=await Promise.all(files.map((file)=>readFile(file,"utf8")));
  assert.ok(sources.slice(0,4).every((source)=>source.includes("export async function GET")&&!source.includes("export async function POST")));
  assert.match(sources[2],/encodeURIComponent\(accountId\)/);
  assert.match(sources[4],/new ReadableStream/);
  assert.doesNotMatch(sources[4],/\.blob\(|\.arrayBuffer\(/);
  assert.doesNotMatch(sources[5],/(?:child_process|spawn\(|exec\(|shell\s*:)/i);
  assert.doesNotMatch(sources[6],/name=["'](?:path|filename|url)["']/i);
});
