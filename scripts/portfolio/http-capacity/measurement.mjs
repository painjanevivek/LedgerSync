import { percentiles } from './driver.mjs';
// One rule set is used by the driver and by the independent file verifier.
export function assessSteadyMeasurement(steady, samples, targets, technicalAttempts) {
    const latency = percentiles(steady.results.map(r => r.latency_ms));
    const failures = steady.results.filter(r => r.technical_failure).length;
    const live = targets.filter(t => !['seed', 'migrate'].includes(t.service));
    const finite = value => Number.isFinite(value) && value >= 0;
    const resourceSample = s => s.status === 'sampled' && Array.isArray(s.services) && s.services.length === live.length && new Set(s.services.map(v => v.id)).size === live.length && live.every(t => s.services.some(v => v.id === t.id && v.service === t.service && [v.cpu_percent, v.working_set_bytes, v.process_rss_bytes_sum].every(finite))) && s.database && ['pending', 'oldest_seconds', 'dead', 'connections', 'lock_waiters', 'database_bytes'].every(k => finite(s.database[k])) && Array.isArray(s.pool_and_policy_metrics);
    const resources = samples.length >= 2 && samples.every(resourceSample) && samples.some(s => s.pool_and_policy_metrics.some(l => l.startsWith('ledgersync_database_pool_wait'))) && samples.some(s => s.pool_and_policy_metrics.some(l => l.startsWith('ledgersync_database_policy_lock')));
    const constraint = steady.stop_constraint || (steady.observed_seconds < 300 || steady.offered !== 3000 ? 'incomplete_steady_interval' : failures || technicalAttempts ? 'technical_failures' : steady.dropped ? 'generator_dropped' : steady.generator_max_lag_ms > 500 ? 'generator_lag' : latency.p95_ms > 500 ? 'command_p95' : !resources ? 'resource_evidence' : samples.some(s => s.database.dead > 0 || s.database.pending > 1000 || s.database.oldest_seconds > 5 || s.database.database_bytes > 256 * 1024 * 1024) ? 'queue_or_database_budget' : null);
    return {
        ...latency,
        technical_failures: failures,
        capacity_qualified: constraint === null,
        stop_constraint: constraint,
        resource_samples: samples.filter(s => s.status === 'sampled').length
    };
}
