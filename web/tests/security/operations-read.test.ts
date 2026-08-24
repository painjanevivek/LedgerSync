import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { NextRequest } from "next/server";

import { sanitizeEventDetail, sanitizeEventPage, sanitizeLocalDiagnostics, sanitizeOperationsBody } from "../../src/lib/api/operations";
import { authorizeOperationsRead, isOperationsReadDenial, readBoundedOperationsResponse, strictOperationsQuery } from "../../src/lib/operations-read";
import type { RateLimitStore } from "../../src/lib/rate-limit";
import type { Session } from "../../src/lib/session";

const origin = "http://127.0.0.1:3000";
const session: Session = { subjectId:"operator-1",tenantId:"tenant-1",csrfToken:"csrf",expiresAt:Date.now()+60_000,scopes:["local:read","events:read"] };
const allow: RateLimitStore = { consume: async () => ({ allowed:true,retryAfterSeconds:0 }) };
const event = { event_id:"77777777-7777-4777-8777-777777777777",event_type:"account.balance.changed.v1",state:"dead",aggregate_type:"account",aggregate_id:"22222222-2222-4222-8222-222222222222",aggregate_version:"4",attempt_count:"2",occurred_at:"2026-08-19T11:00:01Z",available_at:"2026-08-19T11:02:00Z",transfer_id:"33333333-3333-4333-8333-333333333333",correlation_id:"88888888-8888-4888-8888-888888888888",last_error_code:"redis_unavailable" };

function request(path = "/api/events?limit=25", method = "GET", host = "127.0.0.1:3000") { return new NextRequest(`${origin}${path}`, { method, headers:{ host } }); }

test.beforeEach(() => { process.env.LEDGERSYNC_PUBLIC_ORIGIN = origin; });

test("operations reads require signed scope, exact Host, GET, and a bounded tenant-operator rate limit", async () => {
  assert.equal(isOperationsReadDenial(await authorizeOperationsRead(request(), session, "events:read", allow)), false);
  const denied = [
    await authorizeOperationsRead(request(), null, "events:read", allow),
    await authorizeOperationsRead(request(), { ...session, scopes:["local:read"] }, "events:read", allow),
    await authorizeOperationsRead(request(undefined, "GET", "attacker.example:3000"), session, "events:read", allow),
    await authorizeOperationsRead(request(undefined, "POST"), session, "events:read", allow),
  ];
  assert.deepEqual(denied.map((value) => isOperationsReadDenial(value) ? value.status : 0), [401,403,400,405]);
  let key = "";
  const limited: RateLimitStore = { consume: async (value) => { key = value; return { allowed:false,retryAfterSeconds:9 }; } };
  const response = await authorizeOperationsRead(request(), session, "events:read", limited);
  assert.equal(key, "operations:events:read:tenant-1:operator-1");
  assert.equal(isOperationsReadDenial(response) ? response.status : 0, 429);
  assert.equal(isOperationsReadDenial(response) ? response.headers.get("Retry-After") : null, "9");
});

test("event queries reject unknown, duplicate, oversized, malformed, and reversed filters", () => {
  const allowed = ["eventType","state","relatedId","correlationId","from","to","cursor","limit"];
  assert.ok(strictOperationsQuery(request("/api/events?eventType=account.balance.changed.v1&state=dead&limit=25"), allowed) instanceof URLSearchParams);
  for (const path of [
    "/api/events?tenantId=attacker", "/api/events?state=dead&state=pending", "/api/events?limit=101",
    "/api/events?state=unknown", "/api/events?relatedId=not-a-uuid", "/api/events?correlationId=secret",
    "/api/events?from=2026-08-20T00:00:00Z&to=2026-08-19T00:00:00Z",
  ]) assert.equal(strictOperationsQuery(request(path), allowed) instanceof URLSearchParams, false, path);
});

test("diagnostics sanitizer preserves truthful partial unavailable domains and strips extra infrastructure", () => {
  const partial = { overall_state:"unavailable",generated_at:"2026-08-19T12:00:00Z",application:{version:"test",commit:"abc123",environment:"local_demo"},financial_authority:{postgres:{state:"unavailable"},latest_reconciliation:{state:"unavailable"}},delivery_cache:{outbox:{state:"unavailable",worker_progress:"unknown"},redis:{state:"reachable",label:"disposable_cache"}} };
  const result = sanitizeLocalDiagnostics(200, partial, origin);
  assert.equal(result.status, 200);
  assert.deepEqual((result.body.financial_authority as {postgres:unknown}).postgres, { state:"unavailable" });
  assert.equal(JSON.stringify(result.body).includes("public_origin"), true);
  assert.equal(sanitizeLocalDiagnostics(200, { ...partial, dsn:"postgres://secret" }).status, 503);
  assert.equal(sanitizeLocalDiagnostics(200, { ...partial, overall_state:"ready" }).status, 503);
});

test("event sanitizers preserve bounded evidence and reject payloads, endpoint details, raw errors, and secret-like codes", () => {
  assert.equal(sanitizeEventPage(200, { events:[event],next_cursor:"cursor" }).status, 200);
  const detail = { ...event,delivery_attempts:[{attempt_id:"99999999-9999-4999-8999-999999999999",kind:"notification",state:"dead",attempt_number:"2",due_at:"2026-08-19T11:02:00Z",error_code:"timeout"}],delivery_attempts_truncated:true,timeline:[{kind:"delivery_dead",occurred_at:"2026-08-19T11:02:00Z"}] };
  const result = sanitizeEventDetail(200, detail);
  assert.equal(result.status, 200);
  assert.equal(result.body.delivery_attempts_truncated, true);
  for (const hostile of [{ ...event,payload:{token:"secret"} },{ ...event,endpoint:"https://internal" },{ ...event,last_error_code:"client_secret" }]) assert.equal(sanitizeEventPage(200, { events:[hostile],next_cursor:"" }).status, 503);
  assert.equal(sanitizeEventDetail(200, { ...detail,delivery_attempts:[{...detail.delivery_attempts[0],error_code:"bearer_token"}] }).status, 503);
  assert.equal(sanitizeEventDetail(200, { ...detail,delivery_attempts:[{...detail.delivery_attempts[0],kind:"internal_queue"}] }).status, 503);
});

test("upstream parsing is capped before decode and malformed or oversized bodies fail closed", async () => {
  await assert.rejects(() => readBoundedOperationsResponse(new Response("{}", { headers:{ "content-length":"262145" } })), /response_too_large/);
  const oversized = new ReadableStream<Uint8Array>({ start(controller) { controller.enqueue(new Uint8Array(262_145)); controller.close(); } });
  await assert.rejects(() => readBoundedOperationsResponse(new Response(oversized)), /response_too_large/);
  assert.equal(sanitizeOperationsBody(200, "not-json", sanitizeEventPage).status, 503);
  assert.equal(sanitizeOperationsBody(200, "x".repeat(262_145), sanitizeEventPage).status, 503);
});

test("operations route surface is read-only and uses the fixed private paths", async () => {
  const root = process.cwd();
  const paths = ["src/app/api/local/diagnostics/route.ts","src/app/api/events/route.ts","src/app/api/events/[eventId]/route.ts"];
  const sources = await Promise.all(paths.map((path) => readFile(`${root}/${path}`, "utf8")));
  assert.ok(sources.every((source) => source.includes("export async function GET") && !source.includes("export async function POST")));
  assert.match(sources[0], /\/api\/local\/diagnostics/);
  assert.match(sources[1], /\/api\/events/);
  assert.match(sources[2], /encodeURIComponent\(eventId\)/);
});
