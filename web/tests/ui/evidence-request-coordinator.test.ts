import assert from "node:assert/strict";
import { test } from "node:test";

import {
  appendUniqueBy,
  beginEvidenceRequest,
  createEvidenceRequestCoordinator,
  finishEvidenceRequest,
  invalidateEvidenceRequests,
  isEvidenceRequestCurrent,
} from "../../src/features/console/evidenceRequestCoordinator";

test("a newer replacement invalidates an older response for the same resource", () => {
  const coordinator = createEvidenceRequestCoordinator();
  const first = beginEvidenceRequest(coordinator, "transfers:q=wire")!;
  const second = beginEvidenceRequest(coordinator, "transfers:q=wire")!;
  assert.equal(isEvidenceRequestCurrent(coordinator, first.token), false);
  assert.equal(isEvidenceRequestCurrent(coordinator, second.token), true);
});

test("append is single-flight and bound to the verified query identity", () => {
  const coordinator = createEvidenceRequestCoordinator();
  beginEvidenceRequest(coordinator, "corrections:status=requested");
  const page = beginEvidenceRequest(coordinator, "corrections:status=requested", "append")!;
  assert.equal(beginEvidenceRequest(coordinator, "corrections:status=requested", "append"), null);
  assert.equal(beginEvidenceRequest(coordinator, "corrections:status=approved", "append"), null);
  assert.equal(finishEvidenceRequest(coordinator, page.token), true);
  assert.ok(beginEvidenceRequest(coordinator, "corrections:status=requested", "append"));
});

test("route invalidation rejects late work and immutable identifiers deduplicate page overlap", () => {
  const coordinator = createEvidenceRequestCoordinator();
  const request = beginEvidenceRequest(coordinator, "funding:list")!;
  invalidateEvidenceRequests(coordinator);
  assert.equal(isEvidenceRequestCurrent(coordinator, request.token), false);
  assert.deepEqual(
    appendUniqueBy([{ id: "a" }], [{ id: "a" }, { id: "b" }, { id: "b" }], (item) => item.id),
    [{ id: "a" }, { id: "b" }],
  );
});
