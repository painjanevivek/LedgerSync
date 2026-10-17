import { createHash } from 'node:crypto';
import { readFileSync, lstatSync, realpathSync, writeFileSync, chmodSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { isDeepStrictEqual } from 'node:util';
import { capacityCaseDefinition, validateImageSet } from './definition.mjs';
import { assertCapacityTarget, validateFinancialSnapshot } from './boundary.mjs';
import { percentiles } from './driver.mjs';
import { assessSteadyMeasurement } from './measurement.mjs';
export function createCaseWriter(root, limit = 8 * 1024 * 1024) {
    if (!Number.isInteger(limit) || limit < 128 || limit > 8 * 1024 * 1024)
        throw Error('Bounded case evidence limit required');
    let used = 0, files = 0;
    return (name, value) => {
        if (!/^[a-zA-Z0-9.-]+$/.test(name))
            throw Error('Unsafe evidence name');
        const text = typeof value === 'string' ? value : JSON.stringify(value, null, 2) + '\n';
        const size = Buffer.byteLength(text), secret = ['runtime.env', 'compose.private.json'].includes(name), reserved = ['failure.json', 'cleanup.json', 'artifacts.json'].includes(name), ceiling = reserved ? limit : limit - Math.min(65536, Math.floor(limit / 4));
        if (files >= 40 || size > limit || !secret && used + size > ceiling)
            throw Error('Capacity evidence size or file bound exceeded');
        writeFileSync(join(root, name), text, {
            flag: 'wx',
            mode: 0o600
        });
        // Only these two synthetic, credential-free inputs are mounted into
        // non-root containers. Secret files and all other evidence stay 0600.
        if(['fixture.sql','collector.json'].includes(name))chmodSync(join(root,name),0o644);
        files++;
        if (!secret)
            used += size;
    };
}
// Hashes bind local files. They do not authenticate a fabricated experiment.
// Review source, the capture host and command provenance before publication.
export function verifyCapacityCaseEvidence(root, { sourceSHA, machineSHA, images }) {
    validateImageSet(images);
    const canonical = realpathSync(root);
    const bytes = name => {
        if (!/^[a-zA-Z0-9.-]+$/.test(name) || name === 'runtime.env' || name === 'compose.private.json')
            throw Error('Private or unsafe artifact path');
        const path = join(root, name), info = lstatSync(path);
        if (!info.isFile() || info.isSymbolicLink() || dirname(realpathSync(path)) !== canonical || info.size > 8 * 1024 * 1024)
            throw Error('Artifact escaped the bounded case directory');
        return readFileSync(path);
    };
    const manifest = JSON.parse(bytes('artifacts.json'));
    if (!Array.isArray(manifest.files) || manifest.files.length > 40 || new Set(manifest.files.map(f => f.path)).size !== manifest.files.length)
        throw Error('Unique bounded artifact manifest required');
    let total = 0;
    for (const file of manifest.files) {
        const value = bytes(file.path);
        total += value.length;
        if (file.bytes !== value.length || file.sha256 !== createHash('sha256').update(value).digest('hex'))
            throw Error('Capacity artifact hash differs');
    }
    if (total > 8 * 1024 * 1024)
        throw Error('Capacity case exceeded the eight MiB evidence bound');
    const required = ['definition.json', 'report.json', 'financial.json', 'stages.json', 'steady-resources.json', 'enforcement.json', 'cleanup.json'];
    if (required.some(name => !manifest.files.some(file => file.path === name)))
        throw Error('Runtime evidence is incomplete');
    const json = name => JSON.parse(bytes(name)), report = json('report.json'), selected = capacityCaseDefinition(report.id);
    if (report.evidence_kind !== 'runtime_capture' || report.source_manifest_sha256 !== sourceSHA || report.machine_sha256 !== machineSHA || !isDeepStrictEqual(report.images, images) || !isDeepStrictEqual(json('definition.json'), selected))
        throw Error('Capacity source, images, machine or method differs');
    if (!Array.isArray(report.counts?.expected_balances))
        throw Error('Independent expected balances required');
    validateFinancialSnapshot(json('financial.json'), report.counts);
    if (report.financial_invariants !== true || report.unresolved_outcomes !== 0 || report.counts.unresolved !== 0)
        throw Error('Financial acceptance is incomplete');
    const enforced = json('enforcement.json'), budgets = {
        ...Object.fromEntries(Array.from({
            length: selected.api_instances
        }, (_, i) => [i ? 'api-peer' : 'api', {
                cpu_millicores: selected.allocation.cpu_millicores_per_replica,
                memory_mib: selected.allocation.memory_mib_per_replica
            }])),
        postgres: selected.fixed_services.postgres,
        redis: selected.fixed_services.redis,
        'outbox-worker': selected.fixed_services.worker,
        telemetry: selected.fixed_services.telemetry,
        'api-router': selected.fixed_services.router,
        'web-0': {
            cpu_millicores: selected.bff_budget.cpu_millicores / 2,
            memory_mib: selected.bff_budget.memory_mib / 2
        },
        'web-1': {
            cpu_millicores: selected.bff_budget.cpu_millicores / 2,
            memory_mib: selected.bff_budget.memory_mib / 2
        },
        migrate: {
            cpu_millicores: 1000,
            memory_mib: 512
        },
        seed: {
            cpu_millicores: 1000,
            memory_mib: 256
        }
    };
    if (!Array.isArray(enforced) || enforced.length !== Object.keys(budgets).length || new Set(enforced.map(t => t.expected.service)).size !== enforced.length)
        throw Error('All exact service enforcement records required');
    for (const { expected, observed } of enforced) {
        const image = expected.service === 'seed' ? images.migrate : expected.service === 'api-peer' ? images.api : expected.service.startsWith('web-') || expected.service === 'api-router' ? images.web : images[expected.service];
        if (!isDeepStrictEqual(budgets[expected.service], {
            cpu_millicores: expected.cpu_millicores,
            memory_mib: expected.memory_mib
        }) || expected.sourceSHA !== sourceSHA || expected.image !== image)
            throw Error('Recorded allocation differs');
        assertCapacityTarget(observed, expected, {
            allowStopped: ['seed', 'migrate'].includes(expected.service)
        });
    }
    const cleanup = json('cleanup.json');
    if (cleanup.length !== enforced.length || enforced.some(t => !cleanup.some(c => c.id === t.expected.id && c.removed === true)))
        throw Error('Owned cleanup is incomplete');
    const stages = json('stages.json'), steady = stages.find(s => s.name === 'steady'), resources = json('steady-resources.json');
    if (!steady || !isDeepStrictEqual(steady.results, report.steady.results) || !isDeepStrictEqual(percentiles(steady.results.map(r => r.latency_ms)), {
        p50_ms: report.p50_ms,
        p95_ms: report.p95_ms,
        p99_ms: report.p99_ms
    }) || report.resource_samples !== resources.filter(s => s.status === 'sampled').length)
        throw Error('Report differs from raw measurement');
    const posted = new Set(steady.results.filter(r => r.kind === 'new' && r.status === 'posted').map(r => r.key)).size;
    if (report.observed_steady_seconds !== steady.observed_seconds || report.new_posted_per_second !== posted / steady.observed_seconds || report.technical_failures !== steady.results.filter(r => r.technical_failure).length || report.dropped !== steady.dropped || steady.completed !== steady.results.length || steady.offered !== steady.completed + steady.dropped)
        throw Error('Reported command accounting differs');
    if (report.capacity_qualified && (steady.observed_seconds < 300 || steady.offered !== 3000 || report.technical_failures !== 0 || report.technical_attempt_failures !== 0 || report.dropped !== 0 || report.p95_ms > 500 || steady.generator_max_lag_ms > 500 || report.resource_samples < 2 || resources.some(s => s.status !== 'sampled') || report.stop_constraint !== null))
        throw Error('An incomplete or unfavourable run cannot qualify');
    const assessment = assessSteadyMeasurement(steady, resources, enforced.map(t => t.expected), report.technical_attempt_failures);
    if (Object.entries(assessment).some(([key, value]) => report[key] !== value))
        throw Error('Qualification differs from raw resources or constraints');
    return {
        id: report.id,
        verified_local_artifacts: true,
        artifact_manifest_sha256: createHash('sha256').update(bytes('artifacts.json')).digest('hex'),
        case_bytes: total,
        report
    };
}
