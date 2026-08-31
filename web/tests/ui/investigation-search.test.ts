import assert from "node:assert/strict";
import test from "node:test";

import { investigationResultHref } from "../../src/features/investigation/InvestigationSearchController";
import { investigationSearchURL, parseInvestigationSearchPageQuery } from "../../src/lib/page-query/investigation-search";

const identifier = "11111111-1111-4111-8111-111111111111";

test("search page URLs retain only one exact non-secret lookup", () => {
  assert.deepEqual(parseInvestigationSearchPageQuery({ q: identifier.toUpperCase() }), { query: identifier, queryKind: "immutable_id" });
  assert.deepEqual(parseInvestigationSearchPageQuery({ q: "approved-reference" }), { query: "approved-reference", queryKind: "approved_reference" });
  assert.deepEqual(parseInvestigationSearchPageQuery({}), { query: "", queryKind: null });
  assert.equal(parseInvestigationSearchPageQuery({ q: "short" }), null);
  assert.equal(parseInvestigationSearchPageQuery({ q: [identifier, identifier] }), null);
  assert.equal(parseInvestigationSearchPageQuery({ q: identifier, tenant: "other" }), null);
  assert.equal(investigationSearchURL(identifier), `/search?q=${identifier}`);
});

test("typed locators open only canonical released detail routes", () => {
  const base = { record_id: identifier, safe_label: "Transfer", status: "posted", occurred_at: "2026-09-01T10:00:00Z", source: "postgresql" as const, freshness: "search_snapshot" as const };
  assert.equal(investigationResultHref({ ...base, record_type: "transfer" }), `/transfers/${identifier}`);
  assert.equal(investigationResultHref({ ...base, record_type: "request_reference", related_record_type: "account", related_record_id: identifier }), `/accounts/${identifier}`);
  assert.equal(investigationResultHref({ ...base, record_type: "request_reference" }), null);
});
