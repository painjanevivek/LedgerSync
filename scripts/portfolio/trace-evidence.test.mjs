import test from 'node:test';
import assert from 'node:assert/strict';
import { verifyCommandTrace } from './trace-evidence.mjs';

const fixture = () => ({ schema: 'ledgersync-command-trace/v1', evidence_kind: 'runtime_capture', source_manifest_sha256: '1'.repeat(64), financial_evidence_sha256: '2'.repeat(64), captured_at: '2026-10-17T12:00:00Z', spans: ['bff.private_context', 'http.server', 'db.transfer.submit', 'event.redis.publish', 'webhook.delivery.attempt', 'webhook.delivery.attempt', 'webhook.receiver'].map((name, i) => ({ name, trace_id: '3'.repeat(32), span_id: String(i + 1).padStart(16, '0'), ...(i ? { parent_span_id: String(i === 6 ? 6 : 1).padStart(16, '0') } : {}), links: [], status: i === 4 ? 'error' : 'ok' })) });
test('linked capture requires complete stages, distinct attempts and separate financial proof', () => {
  assert.equal(verifyCommandTrace(fixture()).attempts, 2);
  const linked = fixture(); linked.spans[3].trace_id = '4'.repeat(32); delete linked.spans[3].parent_span_id; linked.spans[3].links = [{ trace_id: linked.spans[0].trace_id, span_id: linked.spans[0].span_id }];
  assert.equal(verifyCommandTrace(linked).status, 'structure_verified');
});

test('malformed links and reachable cycles are rejected', () => {
  for (const links of [undefined, null, [null]]) {
    const capture = fixture(); capture.spans[2].links = links;
    assert.throws(() => verifyCommandTrace(capture), /[Mm]alformed/);
  }
  const capture = fixture();
  capture.spans[2].links.push({trace_id: capture.spans[3].trace_id, span_id: capture.spans[3].span_id});
  capture.spans[3].links.push({trace_id: capture.spans[2].trace_id, span_id: capture.spans[2].span_id});
  assert.throws(() => verifyCommandTrace(capture), /Cyclic/);
});
test('fixtures, private fields, incomplete or disconnected captures cannot pass', () => {
  for (const change of [d => { d.evidence_kind = 'test_fixture'; }, d => { d.spans.pop(); }, d => { d.spans[2].amount = 'private'; }, d => { delete d.spans[3].parent_span_id; }, d => { d.spans[3].parent_span_id = 'f'.repeat(16); }, d => { d.spans[4].status = 'ok'; }, d => { d.spans[1].span_id = d.spans[0].span_id; }, d => { d.spans[3].links = [{ trace_id: 'f'.repeat(32), span_id: 'f'.repeat(16) }]; }]) {
    const document = fixture(); change(document); assert.throws(() => verifyCommandTrace(document));
  }
});
