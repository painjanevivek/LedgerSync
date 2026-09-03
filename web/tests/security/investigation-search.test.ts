import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { NextRequest } from "next/server";

import { readBoundedInvestigationSearchBody, sanitizeInvestigationSearch } from "../../src/lib/api/investigation-search";
import { authorizeInvestigationSearch, isInvestigationSearchDenial, parseInvestigationSearchQuery } from "../../src/lib/investigation-search-boundary";
import type { RateLimitStore } from "../../src/lib/rate-limit";
import type { Session } from "../../src/lib/session";

const origin = "http://127.0.0.1:3000";
const identifier = "11111111-1111-4111-8111-111111111111";
const session: Session = { subjectId: "operator-1", tenantId: "tenant-1", csrfToken: "csrf", expiresAt: Date.now() + 60_000, roles: ["tenant:operator"], scopes: ["investigation:read", "accounts:read"] };
const allow: RateLimitStore = { consume: async () => ({ allowed: true, retryAfterSeconds: 0 }) };
const result = { record_type: "account", record_id: identifier, safe_label: "Account", status: "active", occurred_at: "2026-09-01T10:00:00Z", source: "postgresql", freshness: "search_snapshot" };

function request(path = `/api/investigation/search?q=${identifier}&limit=10`, method = "GET", host = "127.0.0.1:3000") {
  return new NextRequest(`${origin}${path}`, { method, headers: { host } });
}

test.beforeEach(() => { process.env.LEDGERSYNC_PUBLIC_ORIGIN = origin; });

test("search authorization requires role, dedicated scope, readable domain, exact host, GET, and bounded rate", async () => {
  assert.equal(isInvestigationSearchDenial(await authorizeInvestigationSearch(request(), session, allow)), false);
  const denied = [
    await authorizeInvestigationSearch(request(), null, allow),
    await authorizeInvestigationSearch(request(), { ...session, roles: [] }, allow),
    await authorizeInvestigationSearch(request(), { ...session, scopes: ["accounts:read"] }, allow),
    await authorizeInvestigationSearch(request(), { ...session, scopes: ["investigation:read"] }, allow),
    await authorizeInvestigationSearch(request(undefined, "GET", "attacker.example:3000"), session, allow),
    await authorizeInvestigationSearch(request(undefined, "POST"), session, allow),
  ];
  assert.deepEqual(denied.map((value) => isInvestigationSearchDenial(value) ? value.status : 0), [401, 403, 403, 403, 400, 405]);
  let key = "";
  const limited: RateLimitStore = { consume: async (value) => { key = value; return { allowed: false, retryAfterSeconds: 8 }; } };
  const response = await authorizeInvestigationSearch(request(), session, limited);
  assert.equal(key, "investigation:search:tenant-1:operator-1");
  assert.equal(isInvestigationSearchDenial(response) ? response.status : 0, 429);
});

test("search query accepts one exact UUID or approved reference and rejects discovery-shaped input", () => {
  assert.equal(parseInvestigationSearchQuery(new URLSearchParams(`q=${identifier}&limit=20`))?.queryKind, "immutable_id");
  assert.equal(parseInvestigationSearchQuery(new URLSearchParams("q=approved-reference"))?.queryKind, "approved_reference");
  for (const query of ["q=short", "q=approved%20reference", "q=approved-reference&q=second-ref", "q=approved-reference&limit=21", "q=approved-reference&tenantId=other"]) {
    assert.equal(parseInvestigationSearchQuery(new URLSearchParams(query)), null, query);
  }
});

test("typed search sanitizer retains locators and rejects money, payload, extra fields, and malformed relationships", () => {
  assert.equal(sanitizeInvestigationSearch(200, { results: [result], query_kind: "immutable_id", generated_at: "2026-09-01T10:00:01Z", truncated: false }).status, 200);
  for (const hostile of [
    { ...result, balance_minor: "900" },
    { ...result, payload: { token: "secret" } },
    { ...result, source: "redis" },
    { ...result, related_record_type: "transfer" },
  ]) assert.equal(sanitizeInvestigationSearch(200, { results: [hostile], query_kind: "immutable_id", generated_at: "2026-09-01T10:00:01Z", truncated: false }).status, 503);
});

test("upstream search bodies are byte-bounded before JSON parsing", async () => {
  await assert.rejects(() => readBoundedInvestigationSearchBody(new Response("{}", { headers: { "content-length": "65537" } })), /byte limit/);
  const oversized = new ReadableStream<Uint8Array>({ start(controller) { controller.enqueue(new Uint8Array(65_537)); controller.close(); } });
  await assert.rejects(() => readBoundedInvestigationSearchBody(new Response(oversized)), /byte limit/);
});

test("search route stays read-only, forwards only the strict query, and never persists results in browser storage", async () => {
  const source = await readFile(`${process.cwd()}/src/app/api/investigation/search/route.ts`, "utf8");
  const controller = await readFile(`${process.cwd()}/src/features/investigation/InvestigationSearchController.tsx`, "utf8");
  assert.match(source, /export async function GET/);
  assert.doesNotMatch(source, /export async function (?:POST|PUT|PATCH|DELETE)/);
  assert.match(source, /\/api\/investigation\/search\?\$\{parsed\.query\}/);
  assert.doesNotMatch(controller, /localStorage|sessionStorage|indexedDB/);
});
