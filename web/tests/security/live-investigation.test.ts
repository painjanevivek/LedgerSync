import assert from "node:assert/strict";
import test from "node:test";

import { findingAllowed, parseLiveRoom, parseLiveSignalPage, parseTransferRequestStatus, sanitizeLiveSuccess } from "../../src/lib/api/live-investigation";

const roomId = "12345678-1234-4234-8234-123456789012";
const workspaceId = "12345678-1234-4234-8234-123456789013";
const requestReference = "12345678-1234-4234-8234-123456789014";

test("live collaboration contracts discard unknown server fields", () => {
  const room = parseLiveRoom({ room_id: roomId, investigation_id: workspaceId, request_reference: requestReference, status: "active", participant_role: "owner", peer_bound: true, peer_present: false, version: "2", created_at: "2026-09-08T10:00:00Z", expires_at: "2026-09-08T10:30:00Z", authoritative_request_state: "in_progress", owner_subject_id: "must-not-cross-bff" });
  assert.ok(room);
  assert.equal("owner_subject_id" in room, false);
  assert.equal(parseTransferRequestStatus({ request_reference: requestReference, status: "completed", transfer_id: roomId, checked_at: "2026-09-08T10:01:00Z", idempotency_key: "hidden" })?.status, "completed");
});

test("signal pages are bounded and preserve only the signalling envelope", () => {
  const parsed = parseLiveSignalPage({ cursor: "10-0", signals: [{ id: "10-0", room_id: roomId, role: "peer", sequence: 1, kind: "ice", payload: { candidate: "candidate" }, sent_at: "2026-09-08T10:01:00Z", subject_id: "hidden" }] });
  assert.equal(parsed?.signals.length, 1);
  assert.equal("subject_id" in (parsed?.signals[0] ?? {}), false);
  assert.equal(parseLiveSignalPage({ cursor: "10-*", signals: [] }), null);
});

test("findings cannot contradict authoritative transfer state", () => {
  assert.equal(findingAllowed("confirmed_completed", "completed", roomId), true);
  assert.equal(findingAllowed("confirmed_completed", "in_progress"), false);
  assert.equal(findingAllowed("still_unresolved", "completed", roomId), false);
  assert.equal(findingAllowed("escalated", "unavailable"), true);
});

test("private collaboration success sanitizer rejects malformed payloads", () => {
  assert.equal(sanitizeLiveSuccess("/private/transfer-requests/x/status", "GET", { status: "completed" }), null);
  assert.deepEqual(sanitizeLiveSuccess(`/private/investigation/live-rooms/${roomId}/invite`, "POST", { room_id: roomId, invite_token: "A".repeat(43), expires_at: "2026-09-08T10:10:00Z", secret: "discarded" }), { room_id: roomId, invite_token: "A".repeat(43), expires_at: "2026-09-08T10:10:00Z" });
});
