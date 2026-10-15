import { ROOT_CONTEXT, SpanStatusCode, trace, type Tracer } from "@opentelemetry/api";
import { createDiagnosticTraceParent } from "@/lib/diagnostic-trace";

// This span covers private credential/assertion preparation. Upstream API and
// database spans measure the command. Never inherit browser trace context.
export function beginPrivateContextTrace(tracer: Tracer = trace.getTracer("ledgersync/bff")) {
  const span = tracer.startSpan("bff.private_context", { root: true }, ROOT_CONTEXT);
  const context = span.spanContext();
  const valid = context.traceId !== "0".repeat(32) && context.spanId !== "0".repeat(16);
  const traceparent = valid ? `00-${context.traceId}-${context.spanId}-${(context.traceFlags & 1) === 1 ? "01" : "00"}` : createDiagnosticTraceParent();
  let ended = false;
  return { traceparent, end(failed = false) {
    if (ended) return; ended = true;
    if (failed) span.setStatus({ code: SpanStatusCode.ERROR, message: "private_context_failed" });
    span.end();
  } };
}
