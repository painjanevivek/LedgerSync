import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { NextRequest } from "next/server";

import {
  createSavedViewInput,
  normalizeSavedViewDefinition,
  readBoundedSavedViewBody,
  sanitizeSavedViewPage,
} from "../../src/lib/api/saved-investigation-views";
import { authorizeInvestigationSavedViews, isInvestigationSearchDenial } from "../../src/lib/investigation-search-boundary";
import type { RateLimitStore } from "../../src/lib/rate-limit";
import type { Session } from "../../src/lib/session";

const origin = "http://127.0.0.1:3000";
const identifier = "11111111-1111-4111-8111-111111111111";
const session: Session = { subjectId: "operator-1", tenantId: "tenant-1", csrfToken: "csrf", expiresAt: Date.now() + 60_000, roles: ["tenant:operator"], scopes: ["investigation:read", "investigation:write", "events:read"] };
const allow: RateLimitStore = { consume: async () => ({ allowed: true, retryAfterSeconds: 0 }) };

function request(method = "GET", csrf = false, host = "127.0.0.1:3000") {
  const headers: Record<string, string> = { host };
  if (csrf) { headers.origin = origin; headers["x-csrf-token"] = "csrf"; }
  return new NextRequest(`${origin}/api/investigation/saved-views`, { method, headers });
}

function savedView(overrides: Record<string, unknown> = {}) {
  return {
    saved_view_id: identifier,
    name: "Dead delivery events",
    filter_schema_version: "1",
    domain: "events",
    filters: { state: "dead" },
    target_path: "/events?state=dead",
    version: "1",
    created_at: "2026-09-01T10:00:00Z",
    updated_at: "2026-09-01T10:00:00Z",
    ...overrides,
  };
}

test.beforeEach(() => { process.env.LEDGERSYNC_PUBLIC_ORIGIN = origin; });

test("saved-view reads and writes require operator, investigation, domain, host, CSRF, and independent rate buckets", async () => {
  assert.equal(isInvestigationSearchDenial(await authorizeInvestigationSavedViews(request(), session, allow, false)), false);
  assert.equal(isInvestigationSearchDenial(await authorizeInvestigationSavedViews(request("POST", true), session, allow, true)), false);
  const denied = [
    await authorizeInvestigationSavedViews(request(), null, allow, false),
    await authorizeInvestigationSavedViews(request(), { ...session, roles: [] }, allow, false),
    await authorizeInvestigationSavedViews(request(), { ...session, scopes: ["investigation:read"] }, allow, false),
    await authorizeInvestigationSavedViews(request("POST", true), { ...session, scopes: ["investigation:read", "events:read"] }, allow, true),
    await authorizeInvestigationSavedViews(request("POST"), session, allow, true),
    await authorizeInvestigationSavedViews(request("GET", false, "attacker.example:3000"), session, allow, false),
    await authorizeInvestigationSavedViews(request("PATCH", true), session, allow, true),
  ];
  assert.deepEqual(denied.map((value) => isInvestigationSearchDenial(value) ? value.status : 0), [401, 403, 403, 403, 403, 400, 405]);

  const keys: string[] = [];
  const limited: RateLimitStore = { consume: async (key) => { keys.push(key); return { allowed: false, retryAfterSeconds: 7 }; } };
  assert.equal(isInvestigationSearchDenial(await authorizeInvestigationSavedViews(request(), session, limited, false)), true);
  assert.equal(isInvestigationSearchDenial(await authorizeInvestigationSavedViews(request("POST", true), session, limited, true)), true);
  assert.deepEqual(keys, ["investigation:saved-views-read:tenant-1:operator-1", "investigation:saved-views-write:tenant-1:operator-1"]);
});

test("saved definitions preserve only allowlisted filters and derive their own safe target", () => {
  assert.deepEqual(createSavedViewInput("Dead events", "events", { state: "dead", relatedId: identifier.toUpperCase() }), {
    name: "Dead events",
    filter_schema_version: "1",
    domain: "events",
    filters: { state: "dead", relatedId: identifier },
  });
  assert.equal(normalizeSavedViewDefinition("events", { state: "dead" })?.targetPath, "/events?state=dead");
  assert.equal(normalizeSavedViewDefinition("transfers", { from: "2026-09-01T00:00:00Z", to: "2026-09-01T23:59:59.999Z" })?.targetPath, "/transfers?from=2026-09-01T00%3A00%3A00Z&to=2026-09-01T23%3A59%3A59.999Z");
  for (const hostile of [
    { cursor: "opaque-page" },
    { q: "customer name" },
    { requester: "operator-2" },
    { result: JSON.stringify({ balance_minor: "100" }) },
    { state: "dead", tenantId: "tenant-2" },
  ]) assert.equal(normalizeSavedViewDefinition("events", hostile), null);
});

test("saved-view response sanitizer rejects snapshots, financial fields, forged targets, duplicates, and excess views", () => {
  const page = { views: [savedView()], generated_at: "2026-09-01T10:00:01Z" };
  assert.equal(sanitizeSavedViewPage(200, page).status, 200);
  for (const hostile of [
    savedView({ results: [] }),
    savedView({ balance_minor: "100" }),
    savedView({ filters: { state: "dead", cursor: "opaque" } }),
    savedView({ target_path: "/events?state=published" }),
  ]) assert.equal(sanitizeSavedViewPage(200, { ...page, views: [hostile] }).status, 503);
  assert.equal(sanitizeSavedViewPage(200, { ...page, views: [savedView(), savedView()] }).status, 503);
  assert.equal(sanitizeSavedViewPage(200, { ...page, views: Array.from({ length: 26 }, (_, index) => savedView({ saved_view_id: `${String(index).padStart(8, "0")}-1111-4111-8111-111111111111` })) }).status, 503);
});

test("saved-view bodies and routes remain bounded, conditional, CSRF protected, and free of browser persistence", async () => {
  await assert.rejects(() => readBoundedSavedViewBody(new Response("{}", { headers: { "content-length": "65537" } })), /too large/);
  const collection = await readFile(`${process.cwd()}/src/app/api/investigation/saved-views/route.ts`, "utf8");
  const item = await readFile(`${process.cwd()}/src/app/api/investigation/saved-views/[savedViewId]/route.ts`, "utf8");
  const capture = await readFile(`${process.cwd()}/src/features/investigation/SavedViewCapture.tsx`, "utf8");
  const panel = await readFile(`${process.cwd()}/src/features/investigation/SavedViewsPanel.tsx`, "utf8");
  assert.match(collection, /export async function GET/);
  assert.match(collection, /export async function POST/);
  assert.match(collection, /readBoundedJSON/);
  assert.match(item, /export async function PUT/);
  assert.match(item, /export async function DELETE/);
  assert.match(item, /If-Match/);
  assert.match(collection + item, /authorizeInvestigationSavedViews/);
  assert.doesNotMatch(capture + panel, /localStorage|sessionStorage|indexedDB/);
});
