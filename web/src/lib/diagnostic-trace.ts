// The BFF starts an opaque diagnostic context. Browser headers and session
// identifiers are never used to form it. It grants no request authority.
export function createDiagnosticTraceParent(): string {
  const trace = crypto.randomUUID().replaceAll("-", "");
  const span = crypto.randomUUID().replaceAll("-", "").slice(0, 16);
  return `00-${trace}-${span}-00`;
}

export function diagnosticTraceHeaders(headers: Readonly<Record<string, string>>): Record<string, string> {
  const value = headers.traceparent;
  if (!value || !/^00-[a-f0-9]{32}-[a-f0-9]{16}-0[01]$/.test(value)) return {};
  if (value.slice(3, 35) === "0".repeat(32) || value.slice(36, 52) === "0".repeat(16)) return {};
  return { traceparent: value };
}
