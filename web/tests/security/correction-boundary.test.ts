import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { NextRequest } from "next/server";

import {
  isCorrectionID,
  isCorrectionIdempotencyKey,
  toPrivateCorrectionDecision,
  toPrivateCorrectionRequest,
} from "../../src/lib/api/corrections";
import {
  authorizeCorrectionMutation,
  isCorrectionDenial,
} from "../../src/lib/correction-boundary";
import { safeReturnTo } from "../../src/lib/oidc";
import type { Session } from "../../src/lib/session";

const origin = "http://127.0.0.1:3000";
const session: Session = {
  subjectId: "operator-1",
  tenantId: "tenant-1",
  csrfToken: "csrf-correction",
  expiresAt: Date.now() + 60_000,
  authenticatedAt: Date.now(),
  roles: ["tenant:operator"],
  scopes: ["corrections:read", "corrections:write", "corrections:approve"],
};

function request(method = "POST", overrides: Record<string, string> = {}) {
  return new NextRequest(`${origin}/api/transfers/transfer-1/corrections`, {
    method,
    headers: {
      host: "127.0.0.1:3000",
      origin,
      "content-type": "application/json",
      "x-csrf-token": session.csrfToken,
      "idempotency-key": "correction-command-0001",
      ...overrides,
    },
  });
}

test.beforeEach(() => {
  process.env.LEDGERSYNC_PUBLIC_ORIGIN = origin;
});

test("correction inputs accept only the published taxonomy and exact fields", () => {
  assert.deepEqual(
    toPrivateCorrectionRequest({
      reasonCode: "operational_error",
      operatorNote: " verified evidence ",
    }),
    { reason_code: "operational_error", operator_note: "verified evidence" },
  );
  assert.throws(() =>
    toPrivateCorrectionRequest({
      reasonCode: "invented",
      operatorNote: "verified",
    }),
  );
  assert.throws(() =>
    toPrivateCorrectionRequest({
      reasonCode: "duplicate",
      operatorNote: "verified",
      tenantId: "other",
    }),
  );
  assert.deepEqual(
    toPrivateCorrectionDecision({ reason: " independent review " }),
    { reason: "independent review" },
  );
  assert.throws(() =>
    toPrivateCorrectionDecision({ reason: "", approve: true }),
  );
});

test("correction mutation boundary requires method, scope, same-origin CSRF, JSON, and retry identity", () => {
  assert.equal(
    isCorrectionDenial(
      authorizeCorrectionMutation(
        request(),
        session,
        "corrections:write",
        true,
        true,
      ),
    ),
    false,
  );
  assert.equal(
    isCorrectionDenial(
      authorizeCorrectionMutation(
        request("POST", { "content-type": "", "idempotency-key": "" }),
        session,
        "corrections:approve",
        true,
        false,
      ),
    ),
    true,
  );
  const cases = [
    authorizeCorrectionMutation(
      request("DELETE"),
      session,
      "corrections:write",
      true,
      true,
    ),
    authorizeCorrectionMutation(
      request(),
      null,
      "corrections:write",
      true,
      true,
    ),
    authorizeCorrectionMutation(
      request(),
      { ...session, scopes: ["corrections:read"] },
      "corrections:write",
      true,
      true,
    ),
    authorizeCorrectionMutation(
      request("POST", { "x-csrf-token": "" }),
      session,
      "corrections:write",
      true,
      true,
    ),
    authorizeCorrectionMutation(
      request("POST", { origin: "http://attacker.example" }),
      session,
      "corrections:write",
      true,
      true,
    ),
    authorizeCorrectionMutation(
      request("POST", { "content-type": "text/plain" }),
      session,
      "corrections:write",
      true,
      true,
    ),
    authorizeCorrectionMutation(
      request("POST", { "idempotency-key": "short" }),
      session,
      "corrections:write",
      true,
      true,
    ),
  ];
  assert.deepEqual(
    cases.map((value) => (isCorrectionDenial(value) ? value.status : 0)),
    [405, 401, 403, 403, 403, 415, 400],
  );
  assert.equal(
    isCorrectionDenial(
      authorizeCorrectionMutation(
        request("POST", { "content-type": "" }),
        session,
        "corrections:approve",
        false,
        false,
      ),
    ),
    false,
  );
});

test("correction identifiers, retry keys, and authentication returns cannot escape their boundaries", () => {
  assert.equal(isCorrectionID("00000000-0000-4000-8000-000000000001"), true);
  assert.equal(isCorrectionID("../tenant-secret"), false);
  assert.equal(isCorrectionIdempotencyKey("x".repeat(16)), true);
  assert.equal(isCorrectionIdempotencyKey("x".repeat(15)), false);
  assert.equal(
    safeReturnTo("/corrections/correction-1?from=transfer"),
    "/corrections/correction-1?from=transfer",
  );
  assert.equal(safeReturnTo("//attacker.example/path"), "/");
  assert.equal(safeReturnTo("https://attacker.example"), "/");
  assert.equal(safeReturnTo("/\\attacker.example"), "/");
});

test("correction browser surface stays fixed-route and exposes paired evidence", async () => {
  const routes = [
    "transfers/[transferId]/corrections/route.ts",
    "transfer-corrections/route.ts",
    "transfer-corrections/[correctionId]/route.ts",
    "transfer-corrections/[correctionId]/approve/route.ts",
    "transfer-corrections/[correctionId]/reject/route.ts",
    "transfer-corrections/[correctionId]/cancel/route.ts",
    "transfer-corrections/[correctionId]/post/route.ts",
  ];
  const sources = await Promise.all(
    routes.map((path) => readFile(`src/app/api/${path}`, "utf8")),
  );
  assert.ok(sources.every((source) => !/tenant[_-]?id/i.test(source)));
  assert.ok(
    sources.every(
      (source) => !/export async function (?:PUT|PATCH|DELETE)/.test(source),
    ),
  );
  const workspace = await readFile(
    "src/features/corrections/CorrectionsConsole.tsx",
    "utf8",
  );
  const requestPanel = await readFile(
    "src/features/corrections/TransferCorrectionPanel.tsx",
    "utf8",
  );
  assert.match(workspace, /Original · permanent/);
  assert.match(workspace, /Compensation · additive/);
  assert.match(
    `${workspace}\n${requestPanel}`,
    /different\s+authorized (?:operator|subject)/i,
  );
  assert.match(`${workspace}\n${requestPanel}`, /never (?:change|edits?)/i);
  assert.match(sources.at(-1) ?? "", /idempotencyKey/);
});
