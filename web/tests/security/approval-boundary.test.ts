import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { safeInternalReturnPath } from "../../src/lib/navigation";

test("approval BFF is read-only, scope-gated, and query-allowlisted", async () => {
  const route = await readFile("src/app/api/approvals/route.ts", "utf8");
  assert.match(route, /funding:approve/);
  assert.match(route, /corrections:approve/);
  assert.match(route, /proxyPrivateGET\(request, session, "\/api\/approvals", approvalFilters\)/);
  assert.doesNotMatch(route, /export async function (?:POST|PUT|PATCH|DELETE)/);
  for (const field of ["domain", "status", "requester", "age", "requested_after", "requested_before", "actionable_by_me", "cursor", "limit"]) {
    assert.match(route, new RegExp(`"${field}"`));
  }
  for (const forbidden of ["evidence_document", "credential", "secret", "tenant_id"]) {
    assert.doesNotMatch(route, new RegExp(forbidden, "i"));
  }
});

test("approval list contract exposes bounded evidence and no raw document payload", async () => {
  const contract = await readFile("src/lib/api/approvals.ts", "utf8");
  assert.match(contract, /page_count: number/);
  assert.match(contract, /next_cursor\?: string/);
  assert.doesNotMatch(contract, /total_count/);
  assert.doesNotMatch(contract, /payload|document_body|secret|credential/i);
});

test("approval detail return context cannot escape the application origin", () => {
  assert.equal(safeInternalReturnPath("/approvals?domain=funding"), "/approvals?domain=funding");
  assert.equal(safeInternalReturnPath("//attacker.example/path"), undefined);
  assert.equal(safeInternalReturnPath("/\\attacker.example/path"), undefined);
  assert.equal(safeInternalReturnPath("https://attacker.example/path"), undefined);
  assert.equal(safeInternalReturnPath(["/approvals", "/admin"]), undefined);
});
