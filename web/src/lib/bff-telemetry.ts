import { SpanStatusCode } from "@opentelemetry/api";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { resourceFromAttributes } from "@opentelemetry/resources";
import { BatchSpanProcessor, TraceIdRatioBasedSampler, type ReadableSpan, type SpanExporter, type SpanProcessor } from "@opentelemetry/sdk-trace-base";
import { NodeTracerProvider } from "@opentelemetry/sdk-trace-node";
const resource = resourceFromAttributes({ "service.name": "ledgersync-web" });
// Framework auto-spans can contain URLs and error text. Only the named manual
// boundary reaches the exporter, with an explicit empty metadata allowlist.
export class PrivateContextSpanProcessor implements SpanProcessor {
    private readonly batch: BatchSpanProcessor;
    constructor(exporter: SpanExporter) {
        this.batch = new BatchSpanProcessor(exporter, { maxQueueSize: 512, maxExportBatchSize: 64, scheduledDelayMillis: 1000, exportTimeoutMillis: 1000 });
    }
    onStart() { }
    onEnd(span: ReadableSpan) {
        if (span.name !== "bff.private_context" || span.instrumentationScope.name !== "ledgersync/bff")
            return;
        const original = span.spanContext();
        const safe: ReadableSpan = {
            name: "bff.private_context", kind: span.kind,
            spanContext: () => ({ traceId: original.traceId, spanId: original.spanId, traceFlags: original.traceFlags }),
            startTime: span.startTime, endTime: span.endTime, duration: span.duration, ended: true,
            attributes: {}, links: [], events: [], resource,
            status: span.status.code === SpanStatusCode.ERROR ? { code: SpanStatusCode.ERROR, message: "private_context_failed" } : { code: SpanStatusCode.UNSET },
            instrumentationScope: { name: "ledgersync/bff" }, droppedAttributesCount: 0, droppedEventsCount: 0, droppedLinksCount: 0,
        };
        this.batch.onEnd(safe);
    }
    forceFlush() { return this.batch.forceFlush(); }
    shutdown() { return this.batch.shutdown(); }
}
export async function boundedBFFShutdown(provider: Pick<NodeTracerProvider, "shutdown">): Promise<void> {
    let timer: ReturnType<typeof setTimeout> | undefined;
    try {
        await Promise.race([provider.shutdown(), new Promise<void>(resolve => { timer = setTimeout(resolve, 3000); timer.unref(); })]);
    }
    catch { /* Diagnostic loss does not change a financial outcome. */ }
    finally {
        if (timer)
            clearTimeout(timer);
    }
}
export function privateTelemetryURL(endpoint: string | undefined, environment: string): URL {
    if (!endpoint || endpoint.length > 256 || endpoint.trim() !== endpoint)
        throw new Error("private telemetry authority required");
    const target = new URL(endpoint.includes("://") ? endpoint : `http://${endpoint}`);
    if (!["http:", "https:"].includes(target.protocol) || !target.hostname || (!endpoint.includes("://") && !target.port) || target.username || target.password || target.pathname !== "/" || target.search || target.hash)
        throw new Error("invalid private telemetry authority");
    if (environment !== "development" && target.protocol !== "https:")
        throw new Error("verified HTTPS telemetry is required outside development");
    target.pathname = "/v1/traces";
    return target;
}
export function registerBFFTelemetry() {
    if (process.env.LEDGERSYNC_TELEMETRY_ENABLED !== "true")
        return;
    const target = privateTelemetryURL(process.env.LEDGERSYNC_OTLP_HTTP_ENDPOINT, process.env.LEDGERSYNC_DEPLOYMENT_ENV ?? process.env.NODE_ENV ?? "development");
    const exporter = new OTLPTraceExporter({ url: target.href, timeoutMillis: 1000, concurrencyLimit: 1 });
    const provider = new NodeTracerProvider({ resource, spanProcessors: [new PrivateContextSpanProcessor(exporter)], sampler: new TraceIdRatioBasedSampler(0.1), forceFlushTimeoutMillis: 1000, spanLimits: { attributeCountLimit: 0, eventCountLimit: 0, linkCountLimit: 0 } });
    provider.register({ propagator: null, contextManager: null });
    process.once("beforeExit", () => { void boundedBFFShutdown(provider); });
}
