import assert from "node:assert/strict";
import test from "node:test";
import { SpanStatusCode } from "@opentelemetry/api";
import { resourceFromAttributes } from "@opentelemetry/resources";
import { AlwaysOnSampler, InMemorySpanExporter } from "@opentelemetry/sdk-trace-base";
import { NodeTracerProvider } from "@opentelemetry/sdk-trace-node";
import { beginPrivateContextTrace } from "../../src/lib/bff-trace";
import { boundedBFFShutdown, PrivateContextSpanProcessor } from "../../src/lib/bff-telemetry";
test("BFF context is exported as an opaque root without framework or private fields", async () => {
    const exporter = new InMemorySpanExporter();
    const processor = new PrivateContextSpanProcessor(exporter);
    const provider = new NodeTracerProvider({ sampler: new AlwaysOnSampler(), resource: resourceFromAttributes({ email: "private@example.test" }), spanProcessors: [processor] });
    const boundary = beginPrivateContextTrace(provider.getTracer("ledgersync/bff"));
    boundary.end(true);
    boundary.end();
    const hostile = provider.getTracer("ledgersync/bff").startSpan("bff.private_context");
    hostile.setAttribute("amount", "999");
    hostile.addEvent("password=secret");
    hostile.setStatus({ code: SpanStatusCode.ERROR, message: "private SQL detail" });
    hostile.end();
    provider.getTracer("next").startSpan("GET /private-account?token=secret").end();
    await processor.forceFlush();
    const spans = exporter.getFinishedSpans();
    assert.equal(spans.length, 2);
    assert.equal(boundary.traceparent.slice(3, 35), spans[0].spanContext().traceId);
    for (const span of spans) {
        assert.deepEqual(span.attributes, {});
        assert.deepEqual(span.events, []);
        assert.deepEqual(span.links, []);
        assert.equal(span.parentSpanContext, undefined);
        assert.equal(span.spanContext().traceState, undefined);
        assert.deepEqual(span.resource.attributes, { "service.name": "ledgersync-web" });
        assert.equal(span.status.message, "private_context_failed");
    }
    await boundedBFFShutdown(provider);
});
test("BFF shutdown failure remains diagnostic and does not escape into commands", async () => {
    await assert.doesNotReject(boundedBFFShutdown({ shutdown: async () => { throw new Error("private transport error"); } }));
});
// Production BFF export must use normal TLS verification. The Node runtime can
// receive a private CA with NODE_EXTRA_CA_CERTS; no skip-verification option exists.
test("BFF telemetry accepts private HTTPS and rejects authority confusion", async () => {
    const { privateTelemetryURL } = await import("../../src/lib/bff-telemetry");
    assert.equal(privateTelemetryURL("https://collector.example:4318", "production").href, "https://collector.example:4318/v1/traces");
    assert.equal(privateTelemetryURL("127.0.0.1:4318", "development").href, "http://127.0.0.1:4318/v1/traces");
    for (const target of ["collector:4318", "http://collector:4318", "https://user:password@collector:4318", "https://collector:4318/path", "https://collector:4318?token=synthetic", "https://collector:4318#fragment"])
        assert.throws(() => privateTelemetryURL(target, "production"));
});
