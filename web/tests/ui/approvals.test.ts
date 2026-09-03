import assert from "node:assert/strict";
import test from "node:test";

import { approvalQuery, approvalURL } from "../../src/features/approvals/useApprovalWorkspace";
import { approvalDetailHref, emptyApprovalFilters } from "../../src/lib/api/approvals";

test("approval filters serialize with typed status and no fabricated total", () => {
  const filters = {
    ...emptyApprovalFilters,
    domain: "correction" as const,
    status: "correction:requested",
    requester: "operator-1",
    age: "over_24h" as const,
    requestedAfter: "2026-08-01",
    requestedBefore: "2026-08-31",
    actionableByMe: true,
    cursor: "opaque-cursor",
  };
  assert.equal(
    approvalQuery(filters),
    "limit=25&domain=correction&status=correction%3Arequested&requester=operator-1&age=over_24h&requested_after=2026-08-01&requested_before=2026-08-31&actionable_by_me=true&cursor=opaque-cursor",
  );
  assert.equal(
    approvalURL(filters),
    "/approvals?domain=correction&status=correction%3Arequested&requester=operator-1&age=over_24h&requested_after=2026-08-01&requested_before=2026-08-31&actionable_by_me=true&cursor=opaque-cursor",
  );
});

test("approval detail links preserve the exact bounded queue context", () => {
  const returnTo = "/approvals?domain=funding&actionable_by_me=true";
  assert.equal(
    approvalDetailHref({
      domain: "funding",
      record_id: "funding-1",
      requester_subject_id: "operator-1",
      requested_at: "2026-08-31T12:00:00Z",
      age_seconds: "1036800",
      status: "requested",
      amount_minor: "1250",
      currency: "INR",
      evidence_complete: true,
      self_approval_blocked: false,
      actionable_by_me: true,
      required_scope: "funding:approve",
      step_up_status: "not_required",
      safe_next_action: "review_decision",
    }, returnTo),
    "/funding/funding-1?return_to=%2Fapprovals%3Fdomain%3Dfunding%26actionable_by_me%3Dtrue",
  );
});
