import assert from "node:assert/strict";
import test from "node:test";
import { createDiagnosticTraceParent, diagnosticTraceHeaders } from "../../src/lib/diagnostic-trace";
import { accountMutationPrivateHeaders } from "../../src/lib/account-mutation-boundary";
import { reconciliationPrivateHeaders } from "../../src/lib/reconciliation-mutation-boundary";
import { webhookReplayPrivateHeaders } from "../../src/lib/webhook-replay-boundary";
import { traceViewerURL } from "../../src/lib/trace-viewer";

test("BFF diagnostic identities are fresh opaque context and reject private metadata", () => {
  const first = createDiagnosticTraceParent(), second = createDiagnosticTraceParent();
  assert.notEqual(first, second);
  assert.deepEqual(diagnosticTraceHeaders({ traceparent: first, baggage: "amount=100", tracestate: "customer=private" }), { traceparent: first });
  for (const value of ["", "secret", "00-" + "0".repeat(32) + "-1234567890123456-01", first + "\r\nAuthorization: secret"])
    assert.deepEqual(diagnosticTraceHeaders({ traceparent: value }), {});
});

test("trace viewer links use only a fixed configured base and an opaque trace ID", () => {
  const id = "12345678901234567890123456789012";
  assert.equal(traceViewerURL(id, "https://traces.example.test/private")?.href, `https://traces.example.test/private/trace/${id}`);
  assert.equal(traceViewerURL(id, "http://127.0.0.1:16686")?.href, `http://127.0.0.1:16686/trace/${id}`);
  for (const base of [undefined, "http://external.test", "javascript:alert(1)", "https://user:secret@viewer.test", "https://viewer.test?redirect=external", "https://viewer.test/#secret"])
    assert.equal(traceViewerURL(id, base), null);
  for (const unsafe of [undefined, "0".repeat(32), "../../private", id + "?token=secret"])
    assert.equal(traceViewerURL(unsafe, "https://viewer.test"), null);
});

test("strict command header allowlists preserve only valid server diagnostic context", () => {
  const parent = createDiagnosticTraceParent();
  const headers = { Authorization: "Bearer workload", "X-LedgerSync-Actor-Assertion": "assertion", "X-Request-ID": "request", traceparent: parent, baggage: "private", cookie: "private" };
  for (const build of [accountMutationPrivateHeaders, reconciliationPrivateHeaders, webhookReplayPrivateHeaders]) {
    const result = build(headers, "diagnostic-test-request-001");
    assert.equal(result.traceparent, parent); assert.equal(result.baggage, undefined); assert.equal(result.cookie, undefined);
  }
});
