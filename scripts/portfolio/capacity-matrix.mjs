import { isDeepStrictEqual } from 'node:util';

const profiles = ['independent-pairs', 'hot-pair', 'multi-tenant'];
const allocations = [
  { id: 'one-api', replicas: 1, cpu_millicores_per_replica: 1000, memory_mib_per_replica: 512 },
  { id: 'two-api-fixed-total', replicas: 2, cpu_millicores_per_replica: 500, memory_mib_per_replica: 256 },
  { id: 'two-api-per-replica', replicas: 2, cpu_millicores_per_replica: 1000, memory_mib_per_replica: 512 },
];
export function capacityComparisonPlan() {
  return {
    schema: 'ledgersync-capacity-comparison-plan/v1', execution: 'none', qualification: 'pending_runtime',
    scope: 'container HTTP/BFF path; ordinary synthetic transfers; no active holds',
    method: { offered_per_second: 10, steady_seconds: 300, repeats: 3, serial_only: true, wall_seconds_per_case: 600, driver_concurrency: 8, driver_pending_limit: 32, fixed_batch: 10 },
    fixed_services: { worker: { cpu_millicores: 1000, memory_mib: 512 }, postgres: { cpu_millicores: 1000, memory_mib: 768 }, redis: { cpu_millicores: 1000, memory_mib: 256 }, web: { cpu_millicores: 1000, memory_mib: 768 } },
    cases: profiles.flatMap(profile => allocations.flatMap(allocation => [1, 2, 3].map(repeat => ({ id: `${profile}/${allocation.id}/${repeat}`, profile, allocation: { ...allocation }, repeat })))),
    comparisons: profiles.flatMap(profile => [
      { profile, mode: 'fixed_total_api_resources', baseline: 'one-api', candidate: 'two-api-fixed-total' },
      { profile, mode: 'fixed_per_replica_api_resources', baseline: 'one-api', candidate: 'two-api-per-replica' },
    ]),
    required_evidence: ['Frozen source and immutable image identities', 'Exact enforced CPU/memory limits, hardware and background-load identity', 'Complete offered/new/replay/rejection/error counts and latency samples', 'Pool/lock/queue and per-service CPU/RSS observations', 'Financial reconciliation, original-key recovery and artifact hashes'],
    limits: ['This file plans experiments. It starts no process and makes no capacity claim.', 'Native component runs cannot be substituted for this container HTTP/BFF comparison.', 'Run after an explicit runtime decision; keep the normal database and volumes excluded.'],
  };
}

// A source-matched complete cohort is required. Failed performance cases stay
// in the result; missing repeats, changed budgets and failed correctness cannot
// become a passing comparison. Capture provenance still needs human review.
export function summarizeCapacityComparisons(reports) {
  const plan = capacityComparisonPlan();
  if (!Array.isArray(reports) || reports.length !== plan.cases.length) throw Error('Every planned repeat, including failed cases, is required');
  const source = reports[0]?.source_manifest_sha256, machine = reports[0]?.machine_sha256;
  if (!/^[a-f0-9]{64}$/.test(source ?? '') || !/^[a-f0-9]{64}$/.test(machine ?? '')) throw Error('Source and machine identities required');
  const indexed = new Map();
  for (const report of reports) {
    const selected = plan.cases.find(item => item.id === report?.id);
    if (!Number.isFinite(report?.observed_steady_seconds) || report.observed_steady_seconds < 0 || report.observed_steady_seconds > plan.method.wall_seconds_per_case) throw Error('Observed duration required within the case wall bound');
    if (!selected || indexed.has(report.id) || report.schema !== 'ledgersync-http-capacity/v1' || report.evidence_kind !== 'runtime_capture' || report.source_manifest_sha256 !== source || report.machine_sha256 !== machine || !isDeepStrictEqual(report.allocation, selected.allocation) || !isDeepStrictEqual(report.fixed_services, plan.fixed_services) || report.profile !== selected.profile || report.offered_per_second !== 10 || report.requested_steady_seconds !== 300 || report.financial_invariants !== true || report.unresolved_outcomes !== 0 || report.technical_failures !== 0 || report.dropped !== 0 || typeof report.capacity_qualified !== 'boolean' || !Number.isFinite(report.new_posted_per_second) || report.new_posted_per_second < 0 || !Number.isFinite(report.p95_ms) || report.p95_ms < 0 || !Number.isInteger(report.resource_samples) || report.resource_samples < 2 || report.capacity_qualified && report.observed_steady_seconds < 300 || !report.capacity_qualified && !report.stop_constraint) throw Error('Invalid, incomplete, mixed or unsafe comparison cohort');
    indexed.set(report.id, report);
  }
  return {
    schema: 'ledgersync-capacity-comparison-summary/v1', source_manifest_sha256: source, machine_sha256: machine,
    all_cases_qualified: reports.every(report => report.capacity_qualified),
    comparisons: plan.comparisons.map(comparison => ({ ...comparison, cohorts: [comparison.baseline, comparison.candidate].map(allocation => {
      const selected = plan.cases.filter(item => item.profile === comparison.profile && item.allocation.id === allocation).map(item => indexed.get(item.id));
      return { allocation, qualified: selected.every(report => report.capacity_qualified), new_posted_per_second: selected.map(report => report.new_posted_per_second), p95_ms: selected.map(report => report.p95_ms), constraints: selected.map(report => report.stop_constraint ?? null) };
    }) })),
    limits: ['Three repeats describe local variation, not statistical significance or a production SLA.', 'This validates a report cohort; it does not authenticate an invented capture. Retain raw artifact hashes and review provenance.'],
  };
}
