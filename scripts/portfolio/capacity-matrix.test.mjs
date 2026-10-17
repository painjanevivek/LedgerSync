import test from 'node:test';
import assert from 'node:assert/strict';
import { capacityComparisonPlan, summarizeCapacityComparisons } from './capacity-matrix.mjs';

function fixtures() {
  const plan = capacityComparisonPlan();
  return plan.cases.map(item => ({ ...item, schema: 'ledgersync-http-capacity/v1', evidence_kind: 'runtime_capture', source_manifest_sha256: '1'.repeat(64), machine_sha256: '2'.repeat(64), fixed_services: plan.fixed_services, offered_per_second: 10, requested_steady_seconds: 300, observed_steady_seconds: 300, financial_invariants: true, unresolved_outcomes: 0, technical_failures: 0, dropped: 0, capacity_qualified: true, new_posted_per_second: 8.8, p95_ms: 100, resource_samples: 60 }));
}
test('resource modes have different budgets and one shared baseline', () => {
  const plan=capacityComparisonPlan();assert.equal(plan.execution,'none');assert.equal(plan.cases.length,27);
  const allocations=plan.cases.filter(item=>item.profile==='hot-pair'&&item.repeat===1).map(item=>item.allocation);
  assert.equal(allocations[0].replicas*allocations[0].cpu_millicores_per_replica,allocations[1].replicas*allocations[1].cpu_millicores_per_replica);
  assert.equal(allocations[0].replicas*allocations[0].memory_mib_per_replica,allocations[1].replicas*allocations[1].memory_mib_per_replica);
  assert.equal(allocations[0].cpu_millicores_per_replica,allocations[2].cpu_millicores_per_replica);
  assert.equal(summarizeCapacityComparisons(fixtures()).comparisons.length,6);
});

test('missing, nonfinite and impossible observed duration cannot qualify', () => {
  for (const duration of [undefined, NaN, Infinity, -1, 601, 299]) {
    const reports = fixtures(); reports[0].observed_steady_seconds = duration;
    assert.throws(() => summarizeCapacityComparisons(reports));
  }
});
test('failed/flat measurements remain visible; missing repeats and mixed source fail', () => {
  const reports=fixtures();reports[1].capacity_qualified=false;reports[1].stop_constraint='queue_age';reports[1].observed_steady_seconds=17;
  assert.equal(summarizeCapacityComparisons(reports).all_cases_qualified,false);
  for(const edit of [r=>r.pop(),r=>{r[1].source_manifest_sha256='3'.repeat(64);},r=>{r[1].allocation={...r[1].allocation,memory_mib_per_replica:1024};},r=>{r[1].id=r[0].id;},r=>{r[1].resource_samples=0;},r=>{r[1].schema='ledgersync-local-capacity/v1';},r=>{r[1].technical_failures=1;}]) { const input=fixtures();edit(input);assert.throws(()=>summarizeCapacityComparisons(input)); }
});
