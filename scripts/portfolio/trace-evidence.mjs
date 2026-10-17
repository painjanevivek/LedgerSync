// This checks a sanitised projection of a capture, not the truth of a claim.
// Never turn generated fixtures into runtime acceptance evidence.
const hex = (value, width) => typeof value === 'string' && new RegExp(`^[a-f0-9]{${width}}$`).test(value) && !/^0+$/.test(value);
const required = ['bff.private_context', 'http.server', 'db.transfer.submit', 'event.redis.publish', 'webhook.delivery.attempt', 'webhook.receiver'];
export function verifyCommandTrace(document) {
  if (!document || Object.keys(document).some(key=>!['schema','evidence_kind','source_manifest_sha256','financial_evidence_sha256','captured_at','spans'].includes(key)) || document.schema !== 'ledgersync-command-trace/v1' || document.evidence_kind !== 'runtime_capture' || !hex(document.source_manifest_sha256, 64) || !hex(document.financial_evidence_sha256, 64) || !Number.isFinite(Date.parse(document.captured_at)) || !Array.isArray(document.spans) || document.spans.length < 7 || document.spans.length > 1000 || Buffer.byteLength(JSON.stringify(document)) > 1024 * 1024) throw Error('Bounded runtime trace and separate financial evidence required');
  const spans = new Map();
  for (const span of document.spans) {
    if (!span || typeof span !== 'object' || !Array.isArray(span.links)) throw Error('Malformed span');
    if (span.parent_span_id===span.span_id || span.links.some(link=>link?.trace_id===span.trace_id&&link?.span_id===span.span_id)) throw Error('Self-referencing span');
    if (Object.keys(span).some(key => !['name', 'trace_id', 'span_id', 'parent_span_id', 'links', 'status'].includes(key)) || !required.includes(span.name) || !hex(span.trace_id, 32) || !hex(span.span_id, 16) || !['ok', 'error'].includes(span.status) || !Array.isArray(span.links) || span.links.length > 4 || span.parent_span_id && !hex(span.parent_span_id, 16)) throw Error('Unknown, private or malformed span field');
    const key = `${span.trace_id}/${span.span_id}`;
    if (spans.has(key)) throw Error('Duplicate span identity');
    spans.set(key, span);
  }
  for (const name of required) if (!document.spans.some(span => span.name === name)) throw Error(`Missing stage: ${name}`);
  const attempts = document.spans.filter(span => span.name === 'webhook.delivery.attempt');
  if (attempts.length < 2 || !attempts.some(span => span.status === 'error') || !attempts.some(span => span.status === 'ok')) throw Error('Distinct failed and recovered delivery attempts required');
  const root = document.spans.filter(span => span.name === 'bff.private_context');
  if (root.length !== 1 || root[0].parent_span_id || root[0].links.length) throw Error('One BFF origin required');
  const reachable = new Set([`${root[0].trace_id}/${root[0].span_id}`]);
  for (const span of document.spans) {
    for (const link of span.links) if (!link || typeof link !== 'object' || Object.keys(link).some(key => !['trace_id', 'span_id'].includes(key)) || !hex(link.trace_id, 32) || !hex(link.span_id, 16) || !spans.has(`${link.trace_id}/${link.span_id}`)) throw Error('Link target absent or malformed');
    if (span.parent_span_id && !spans.has(`${span.trace_id}/${span.parent_span_id}`)) throw Error('Parent absent from capture');
  }
  const visiting = new Set(), visited = new Set();
  const visit = key => {
    if (visiting.has(key)) throw Error('Cyclic trace graph');
    if (visited.has(key)) return;
    visiting.add(key);
    const span = spans.get(key);
    if (span.parent_span_id) visit(`${span.trace_id}/${span.parent_span_id}`);
    for (const link of span.links) visit(`${link.trace_id}/${link.span_id}`);
    visiting.delete(key); visited.add(key);
  };
  for (const key of spans.keys()) visit(key);
  for (let round = 0; round < document.spans.length; round++) {
    for (const span of document.spans) {
      if (reachable.has(`${span.trace_id}/${span.parent_span_id}`) || span.links.some(link => reachable.has(`${link.trace_id}/${link.span_id}`))) reachable.add(`${span.trace_id}/${span.span_id}`);
    }
  }
  if (reachable.size !== spans.size) throw Error('Disconnected or cyclic trace graph');
  return { status: 'structure_verified', stages: required.length, spans: spans.size, attempts: attempts.length, financial_result: 'requires separate referenced financial evidence', provenance: 'capture origin must be reviewed separately' };
}
