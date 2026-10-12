import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { NextRequest } from "next/server";

import { isFundingEventID, isFundingIdempotencyKey, toPrivateFundingCompensation, toPrivateFundingDecision, toPrivateFundingRequest } from "../../src/lib/api/funding";
import { toPrivateTransferRequest } from "../../src/lib/api/transfers";
import { canonicalUUID } from "../../src/lib/canonical-uuid";
import { authorizeFundingMutation, isFundingDenial } from "../../src/lib/funding-boundary";
import type { Session } from "../../src/lib/session";

const origin = "http://127.0.0.1:3000";
const session: Session = { subjectId: "finance-1", tenantId: "tenant-1", csrfToken: "csrf-funding", expiresAt: Date.now() + 60_000, roles: ["tenant:operator"], scopes: ["funding:read", "funding:write", "funding:approve"] };

function request(method = "POST", overrides: Record<string, string> = {}) {
  return new NextRequest(`${origin}/api/funding-requests`, { method, headers: { host: "127.0.0.1:3000", origin, "content-type": "application/json", "x-csrf-token": session.csrfToken, "idempotency-key": "funding-command-0001", ...overrides } });
}

test.beforeEach(() => { process.env.LEDGERSYNC_PUBLIC_ORIGIN = origin; });

test("funding commands accept only exact string money and declared evidence fields", () => {
  const input = { destinationAccountId: "A0B1C2D3-E4F5-4678-9ABC-DEF012345678", amountMinor: "125050", currency: "inr", externalReference: "wire-001", evidenceReference: "case://wire-001" };
  assert.deepEqual(toPrivateFundingRequest(input), { destination_account_id: "a0b1c2d3-e4f5-4678-9abc-def012345678", amount_minor: "125050", currency: "INR", external_reference: "wire-001", evidence_reference: "case://wire-001" });
  assert.throws(() => toPrivateFundingRequest({ ...input, amountMinor: "1250.50" }));
  assert.throws(() => toPrivateFundingRequest({ ...input, amountMinor: "0125050" }));
  assert.throws(() => toPrivateFundingRequest({ ...input, amountMinor: 125050 as never }));
  assert.deepEqual(toPrivateFundingDecision({ reason: " verified evidence " }), { reason: "verified evidence" });
  assert.throws(() => toPrivateFundingDecision({ reason: "" }));
  assert.deepEqual(toPrivateFundingCompensation({ reasonCode: "external_reversal", operatorNote: " case verified " }), { reason_code: "external_reversal", operator_note: "case verified" });
});

test("UUID boundaries canonicalize case and reject ambiguous text", () => {
  assert.equal(canonicalUUID("A0B1C2D3-E4F5-4678-9ABC-DEF012345678"), "a0b1c2d3-e4f5-4678-9abc-def012345678");
  for (const value of [
    " a0b1c2d3-e4f5-4678-9abc-def012345678",
    "{a0b1c2d3-e4f5-4678-9abc-def012345678}",
    "a0b1c2d3e4f546789abcdef012345678",
    "00000000-0000-0000-0000-000000000000",
    "not-a-uuid",
  ]) assert.equal(canonicalUUID(value), undefined);

  assert.deepEqual(toPrivateTransferRequest({
    sourceAccountId: "A0B1C2D3-E4F5-4678-9ABC-DEF012345678",
    destinationAccountId: "B0B1C2D3-E4F5-4678-9ABC-DEF012345678",
    amount: { currency: "usd", minorUnits: "100" },
  }), {
    source_account_id: "a0b1c2d3-e4f5-4678-9abc-def012345678",
    destination_account_id: "b0b1c2d3-e4f5-4678-9abc-def012345678",
    amount: "1.00",
    currency: "USD",
  });
  assert.throws(() => toPrivateTransferRequest({
    sourceAccountId: "A0B1C2D3-E4F5-4678-9ABC-DEF012345678",
    destinationAccountId: "a0b1c2d3-e4f5-4678-9abc-def012345678",
    amount: { currency: "USD", minorUnits: "100" },
  }));
});

test("funding mutation boundary requires method, scope, same-origin CSRF, JSON, and retry identity", () => {
  assert.equal(isFundingDenial(authorizeFundingMutation(request(), session, "funding:write", true, true)), false);
  const cases = [
    authorizeFundingMutation(request("DELETE"), session, "funding:write", true, true),
    authorizeFundingMutation(request(), null, "funding:write", true, true),
    authorizeFundingMutation(request(), { ...session, scopes: ["funding:read"] }, "funding:write", true, true),
    authorizeFundingMutation(request("POST", { "x-csrf-token": "" }), session, "funding:write", true, true),
    authorizeFundingMutation(request("POST", { origin: "http://attacker.example" }), session, "funding:write", true, true),
    authorizeFundingMutation(request("POST", { "content-type": "text/plain" }), session, "funding:write", true, true),
    authorizeFundingMutation(request("POST", { "idempotency-key": "short" }), session, "funding:write", true, true),
  ];
  assert.deepEqual(cases.map((value) => isFundingDenial(value) ? value.status : 0), [405, 401, 403, 403, 403, 415, 400]);
  assert.equal(isFundingDenial(authorizeFundingMutation(request("POST", { "content-type": "" }), session, "funding:write", false, false)), false);
  assert.equal(isFundingDenial(authorizeFundingMutation(request("POST", { "content-type": "", "idempotency-key": "" }), session, "funding:write", true, false)), true);
});

test("funding identifiers and idempotency values are bounded without reinterpretation", () => {
  assert.equal(isFundingEventID("00000000-0000-4000-8000-000000000001"), true);
  assert.equal(isFundingEventID("../tenant-secret"), false);
  assert.equal(isFundingIdempotencyKey("x".repeat(16)), true);
  assert.equal(isFundingIdempotencyKey("x".repeat(15)), false);
  assert.equal(isFundingIdempotencyKey(`x${"y".repeat(15)}\n`), false);
});

test("funding browser surface is fixed-route and preserves non-custodial language", async () => {
  const routes = ["funding-requests/route.ts", "funding-events/route.ts", "funding-events/[fundingEventId]/route.ts", "funding-events/[fundingEventId]/approve/route.ts", "funding-events/[fundingEventId]/reject/route.ts", "funding-events/[fundingEventId]/post/route.ts", "funding-events/[fundingEventId]/compensations/route.ts", "funding-events/[fundingEventId]/reconciliation/route.ts"];
  const sources = await Promise.all(routes.map((path) => readFile(`src/app/api/${path}`, "utf8")));
  assert.ok(sources.every((source) => !/tenant[_-]?id/i.test(source)));
  assert.ok(sources.every((source) => !/export async function (?:PUT|PATCH|DELETE)/.test(source)));
  const views = await readFile("src/features/funding/FundingViews.tsx", "utf8");
  const flow = await readFile("src/features/funding/FundingRequestFlow.tsx", "utf8");
  assert.match(`${views}\n${flow}`, /external value reference/i);
  assert.match(`${views}\n${flow}`, /does not (?:claim|describe)/i);
  assert.doesNotMatch(`${views}\n${flow}`, /confirm deposit|bank deposit completed/i);
  assert.match(sources[5], /idempotencyKey/);
});
