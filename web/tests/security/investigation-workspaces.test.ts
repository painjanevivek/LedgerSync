import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { NextRequest } from "next/server";

import { parseWorkspaceCreateInput, parseWorkspaceHandoffInput, readBoundedWorkspaceBody, sanitizeWorkspace, sanitizeWorkspacePage } from "../../src/lib/api/investigation-workspaces";
import { authorizeInvestigationWorkspaces, isInvestigationSearchDenial } from "../../src/lib/investigation-search-boundary";
import type { RateLimitStore } from "../../src/lib/rate-limit";
import type { Session } from "../../src/lib/session";

const origin = "http://127.0.0.1:3000";
const identifier = "11111111-1111-4111-8111-111111111111";
const session: Session = { subjectId: "operator-1", tenantId: "tenant-1", csrfToken: "csrf", expiresAt: Date.now() + 60_000, roles: ["tenant:operator"], scopes: ["investigation:read", "investigation:write", "transfers:read"] };
const allow: RateLimitStore = { consume: async () => ({ allowed: true, retryAfterSeconds: 0 }) };

function request(method = "GET", csrf = false, host = "127.0.0.1:3000") { const headers: Record<string, string> = { host }; if (csrf) { headers.origin = origin; headers["x-csrf-token"] = "csrf"; } return new NextRequest(`${origin}/api/investigation/workspaces`, { method, headers }); }

function workspace(overrides: Record<string, unknown> = {}) {
  return {
    investigation_id: identifier, title: "Transfer delivery review", taxonomy: "transfer_delivery", status: "open", version: "1", created_at: "2026-09-01T10:00:00Z", updated_at: "2026-09-01T10:00:00Z",
    historical_context: { query_context: { kind: "immutable_id", record_type: "transfer", value: identifier }, references: [{ relationship_type: "root", record_type: "transfer", record_id: identifier, target_path: `/transfers/${identifier}`, captured_at: "2026-09-01T10:00:00Z" }], withheld_reference_count: 0, history: [{ action: "created", actor_is_current_operator: true, version: "1", status: "open", occurred_at: "2026-09-01T10:00:00Z" }], history_truncated: false },
    current_evidence: { root: { record_type: "transfer", record_id: identifier, safe_label: "Transfer", status: "posted", occurred_at: "2026-09-01T09:59:00Z", source: "postgresql", freshness: "search_snapshot" }, relationships: [], generated_at: "2026-09-01T10:00:01Z", truncated: false, available: true },
    ...overrides,
  };
}

test.beforeEach(() => { process.env.LEDGERSYNC_PUBLIC_ORIGIN = origin; });

test("workspace reads and mutations require operator, investigation, domain, host, CSRF, and bounded rate keys", async () => {
  assert.equal(isInvestigationSearchDenial(await authorizeInvestigationWorkspaces(request(), session, allow, false)), false);
  assert.equal(isInvestigationSearchDenial(await authorizeInvestigationWorkspaces(request("POST", true), session, allow, true)), false);
  const denied = [
    await authorizeInvestigationWorkspaces(request(), null, allow, false),
    await authorizeInvestigationWorkspaces(request(), { ...session, roles: [] }, allow, false),
    await authorizeInvestigationWorkspaces(request(), { ...session, scopes: ["investigation:read"] }, allow, false),
    await authorizeInvestigationWorkspaces(request("POST", true), { ...session, scopes: ["investigation:read", "transfers:read"] }, allow, true),
    await authorizeInvestigationWorkspaces(request("POST"), session, allow, true),
    await authorizeInvestigationWorkspaces(request("GET", false, "attacker.example:3000"), session, allow, false),
    await authorizeInvestigationWorkspaces(request("DELETE", true), session, allow, true),
  ];
  assert.deepEqual(denied.map((value) => isInvestigationSearchDenial(value) ? value.status : 0), [401, 403, 403, 403, 403, 400, 405]);
  const keys: string[] = []; const limited: RateLimitStore = { consume: async (key) => { keys.push(key); return { allowed: false, retryAfterSeconds: 5 }; } };
  assert.equal(isInvestigationSearchDenial(await authorizeInvestigationWorkspaces(request(), session, limited, false)), true);
  assert.equal(isInvestigationSearchDenial(await authorizeInvestigationWorkspaces(request("POST", true), session, limited, true)), true);
  assert.deepEqual(keys, ["investigation:workspaces-read:tenant-1:operator-1", "investigation:workspaces-write:tenant-1:operator-1"]);
});

test("workspace inputs reject notes, copied facts, unsafe titles, forged roots, and malformed recipients", () => {
  const valid = { title: "Transfer delivery review", taxonomy: "transfer_delivery", query_context: { kind: "immutable_id", record_type: "transfer", value: identifier }, root_record: { record_type: "transfer", record_id: identifier } };
  assert.deepEqual(parseWorkspaceCreateInput(valid), valid);
  for (const hostile of [
    { ...valid, notes: "customer balance 100" },
    { ...valid, title: "token=secret" },
    { ...valid, title: "customer@example.test" },
    { ...valid, query_context: { ...valid.query_context, value: "22222222-2222-4222-8222-222222222222" } },
    { ...valid, root_record: { ...valid.root_record, record_type: "account" } },
  ]) assert.throws(() => parseWorkspaceCreateInput(hostile));
  assert.deepEqual(parseWorkspaceHandoffInput({ expected_version: "2", target_subject_id: "operator-2" }), { expected_version: "2", target_subject_id: "operator-2" });
  for (const hostile of [{ expected_version: "0", target_subject_id: "operator-2" }, { expected_version: "2", target_subject_id: " operator-2" }, { expected_version: "2", target_subject_id: "operator\n2" }, { expected_version: "2", target_subject_id: "operator-2", note: "please review" }]) assert.throws(() => parseWorkspaceHandoffInput(hostile));
});

test("workspace sanitizer separates strict historical references from live current evidence", () => {
  assert.equal(sanitizeWorkspace(200, workspace()).status, 200);
  assert.equal(sanitizeWorkspacePage(200, { investigations: [{ investigation_id: identifier, title: "Transfer delivery review", taxonomy: "transfer_delivery", status: "open", version: "1", created_at: "2026-09-01T10:00:00Z", updated_at: "2026-09-01T10:00:00Z" }], generated_at: "2026-09-01T10:00:02Z" }).status, 200);
  for (const hostile of [
    workspace({ balance_minor: "100" }),
    workspace({ notes: "copied evidence" }),
    workspace({ current_evidence: { ...(workspace().current_evidence as object), root: { ...(workspace().current_evidence as { root: object }).root, amount_minor: "100" } } }),
    workspace({ historical_context: { ...(workspace().historical_context as object), references: [{ relationship_type: "root", record_type: "transfer", record_id: identifier, target_path: `/accounts/${identifier}`, captured_at: "2026-09-01T10:00:00Z" }] } }),
    workspace({ historical_context: { ...(workspace().historical_context as object), notes: "not allowed" } }),
  ]) assert.equal(sanitizeWorkspace(200, hostile).status, 503);
});

test("workspace transport is bounded and browser components do not persist case context", async () => {
  await assert.rejects(() => readBoundedWorkspaceBody(new Response("{}", { headers: { "content-length": "262145" } })), /too large/);
  const files = await Promise.all([
    readFile(`${process.cwd()}/src/app/api/investigation/workspaces/route.ts`, "utf8"),
    readFile(`${process.cwd()}/src/app/api/investigation/workspaces/[investigationId]/route.ts`, "utf8"),
    readFile(`${process.cwd()}/src/app/api/investigation/workspaces/[investigationId]/[action]/route.ts`, "utf8"),
    readFile(`${process.cwd()}/src/features/investigation/WorkspaceCapture.tsx`, "utf8"),
    readFile(`${process.cwd()}/src/features/investigation/InvestigationWorkspaceController.tsx`, "utf8"),
  ]);
  const source = files.join("\n");
  assert.match(source, /readBoundedJSON/);
  assert.match(source, /authorizeInvestigationWorkspaces/);
  assert.doesNotMatch(source, /localStorage|sessionStorage|indexedDB/);
});
