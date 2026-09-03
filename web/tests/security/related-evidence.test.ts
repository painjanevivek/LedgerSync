import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { NextRequest } from "next/server";

import { readBoundedRelatedEvidenceBody, sanitizeRelatedEvidence } from "../../src/lib/api/related-evidence";
import { authorizeInvestigationRelationships, isInvestigationSearchDenial } from "../../src/lib/investigation-search-boundary";
import type { RateLimitStore } from "../../src/lib/rate-limit";
import type { Session } from "../../src/lib/session";

const origin = "http://127.0.0.1:3000";
const identifier = "11111111-1111-4111-8111-111111111111";
const session: Session = { subjectId: "operator-1", tenantId: "tenant-1", csrfToken: "csrf", expiresAt: Date.now() + 60_000, roles: ["tenant:operator"], scopes: ["investigation:read", "transfers:read"] };
const allow: RateLimitStore = { consume: async () => ({ allowed: true, retryAfterSeconds: 0 }) };
const relationship = { relationship_type: "transfer_journal", target_type: "journal", target_id: identifier, safe_label: "Journal transaction", status: "recorded", occurred_at: "2026-09-01T10:00:00Z", source: "postgresql", freshness: "relationship_snapshot" };

function request(method = "GET", host = "127.0.0.1:3000") { return new NextRequest(`${origin}/api/investigation/related/transfer/${identifier}`, { method, headers: { host } }); }
test.beforeEach(() => { process.env.LEDGERSYNC_PUBLIC_ORIGIN = origin; });

test("relationship authorization requires the investigation role boundary and has an independent rate bucket", async () => {
  assert.equal(isInvestigationSearchDenial(await authorizeInvestigationRelationships(request(), session, allow)), false);
  let key = "";
  const limited: RateLimitStore = { consume: async (value) => { key = value; return { allowed: false, retryAfterSeconds: 4 }; } };
  const denied = await authorizeInvestigationRelationships(request(), session, limited);
  assert.equal(key, "investigation:relationships:tenant-1:operator-1");
  assert.equal(isInvestigationSearchDenial(denied) ? denied.status : 0, 429);
});

test("typed relationship sanitizer rejects copied financial data, payloads, unknown nodes, and oversized graphs", () => {
  const page = { source_type: "transfer", source_id: identifier, relationships: [relationship], generated_at: "2026-09-01T10:00:01Z", truncated: false };
  assert.equal(sanitizeRelatedEvidence(200, page).status, 200);
  for (const hostile of [
    { ...relationship, amount_minor: "900" },
    { ...relationship, payload: { secret: "token" } },
    { ...relationship, target_type: "customer" },
    { ...relationship, freshness: "live" },
  ]) assert.equal(sanitizeRelatedEvidence(200, { ...page, relationships: [hostile] }).status, 503);
  assert.equal(sanitizeRelatedEvidence(200, { ...page, relationships: Array.from({ length: 21 }, () => relationship) }).status, 503);
});

test("relationship response bodies are byte bounded and the rail does not persist evidence in browser storage", async () => {
  await assert.rejects(() => readBoundedRelatedEvidenceBody(new Response("{}", { headers: { "content-length": "65537" } })), /byte limit/);
  const route = await readFile(`${process.cwd()}/src/app/api/investigation/related/[recordType]/[recordId]/route.ts`, "utf8");
  const rail = await readFile(`${process.cwd()}/src/features/investigation/RelatedEvidenceRail.tsx`, "utf8");
  assert.match(route, /export async function GET/);
  assert.doesNotMatch(route, /export async function (?:POST|PUT|PATCH|DELETE)/);
  assert.doesNotMatch(rail, /localStorage|sessionStorage|indexedDB/);
});
