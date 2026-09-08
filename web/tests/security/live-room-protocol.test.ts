import assert from "node:assert/strict";
import test from "node:test";

import { maximumLiveMessageBytes, parsePeerEnvelope, signalNegotiationID, validLiveMessage } from "../../src/features/investigation/liveRoomProtocol";

const source = "00000000-0000-4000-8000-000000000010";
const destination = "00000000-0000-4000-8000-000000000020";

test("live room accepts bounded plain text but rejects links and oversized messages", () => {
  assert.equal(validLiveMessage("Compare the source entry with the posting."), true);
  assert.equal(validLiveMessage("open https://example.invalid"), false);
  assert.equal(validLiveMessage("x".repeat(maximumLiveMessageBytes + 1)), false);
  assert.equal(parsePeerEnvelope(JSON.stringify({ version: 1, type: "message", id: "00000000-0000-4000-8000-000000000099", text: "No financial action requested." }), "owner")?.type, "message");
  assert.equal(parsePeerEnvelope(JSON.stringify({ version: 1, type: "message", id: "not-canonical", text: "hello" }), "owner"), null);
});

test("submitted intent is accepted only by the invited peer and remains exact", () => {
  const raw = JSON.stringify({ version: 1, type: "intent", intent: { sourceAccountId: source, destinationAccountId: destination, currency: "USD", amountMinor: "1250" } });
  assert.deepEqual(parsePeerEnvelope(raw, "peer"), { type: "intent", intent: { sourceAccountId: source, destinationAccountId: destination, currency: "USD", amountMinor: "1250" } });
  assert.equal(parsePeerEnvelope(raw, "owner"), null);
  assert.equal(parsePeerEnvelope(JSON.stringify({ version: 1, type: "intent", intent: { sourceAccountId: source, destinationAccountId: source, currency: "USD", amountMinor: "1250" } }), "peer"), null);
});

test("peer messages cannot encode application commands", () => {
  const parsed = parsePeerEnvelope(JSON.stringify({ version: 1, type: "message", id: "00000000-0000-4000-8000-000000000099", text: "approved", command: { action: "retry-transfer" } }), "peer");
  assert.deepEqual(parsed, { type: "message", id: "00000000-0000-4000-8000-000000000099", text: "approved" });
  assert.equal("command" in (parsed ?? {}), false);
});

test("reconnection signals require a canonical per-connection negotiation ID", () => {
  const negotiationID = "00000000-0000-4000-8000-000000000088";
  assert.equal(signalNegotiationID({ type: "offer", sdp: "redacted", negotiation_id: negotiationID }), negotiationID);
  assert.equal(signalNegotiationID({ type: "offer", sdp: "redacted" }), "");
  assert.equal(signalNegotiationID({ negotiation_id: "not-canonical" }), "");
  assert.equal(signalNegotiationID(null), "");
});
