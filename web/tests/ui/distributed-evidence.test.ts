import assert from "node:assert/strict";
import test from "node:test";
import { sanitizeDistributedEvidence } from "../../src/lib/api/distributed";
const time = "2026-10-12T00:00:00Z";
const fixture = { api_replicas: { state: "available", sampled_at: time, count: "2" }, worker_publications: { state: "historical", sampled_at: time, count: "2", latest_published_at: time }, scheduler: { state: "active", sampled_at: time, owner_id: "ledgersync/scheduler/node-a", generation: "9007199254740993", expires_at: time } };
test("distributed observations preserve exact counters and independent sample times", () => { assert.equal(sanitizeDistributedEvidence(200, fixture).status, 200); });
test("distributed observations reject private fields and unsupported states", () => {
  for (const bad of [{ ...fixture, api_replicas: { ...fixture.api_replicas, count: "17" } }, { ...fixture, scheduler: { ...fixture.scheduler, password: "hidden" } }, { ...fixture, worker_publications: { ...fixture.worker_publications, state: "active" } }, { ...fixture, scheduler: { ...fixture.scheduler, generation: "9223372036854775808" } }]) assert.equal(sanitizeDistributedEvidence(200, bad).status, 503);
});
test("unavailable observations do not supply a false zero", () => {
  const unavailable = { state: "unavailable", sampled_at: time };
  assert.equal(sanitizeDistributedEvidence(200, { api_replicas: unavailable, worker_publications: unavailable, scheduler: unavailable }).status, 200);
  assert.equal(sanitizeDistributedEvidence(200, { ...fixture, api_replicas: { ...unavailable, count: "0" } }).status, 503);
});
