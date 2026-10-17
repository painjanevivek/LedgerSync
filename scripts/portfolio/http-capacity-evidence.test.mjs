import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync, readFileSync, readdirSync,statSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createHash } from 'node:crypto';
import { capacityCaseDefinition, accountIDs } from './http-capacity/definition.mjs';
import { verifyCapacityCaseEvidence, createCaseWriter } from './http-capacity/evidence.mjs';
import { assertCapacityTarget } from './http-capacity/boundary.mjs';
const sha = value => createHash('sha256').update(value).digest('hex');
function fixture() {
    const root = mkdtempSync(join(tmpdir(), 'ledgersync-capacity-evidence-')), sourceSHA = 'a'.repeat(64), machineSHA = 'b'.repeat(64);
    const images = Object.fromEntries(['api', 'outbox-worker', 'web', 'migrate', 'postgres', 'redis', 'telemetry'].map((s, i) => [s, 'sha256:' + String(i + 1).repeat(64)]));
    const selected = capacityCaseDefinition('hot-pair/one-api/1'), project = 'ledgersync-capacity-20261017210000-1234abcd';
    const budgets = {
        api: {
            cpu_millicores: 1000,
            memory_mib: 512
        },
        postgres: selected.fixed_services.postgres,
        redis: selected.fixed_services.redis,
        'outbox-worker': selected.fixed_services.worker,
        telemetry: selected.fixed_services.telemetry,
        'api-router': selected.fixed_services.router,
        'web-0': {
            cpu_millicores: 500,
            memory_mib: 384
        },
        'web-1': {
            cpu_millicores: 500,
            memory_mib: 384
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
    const enforced = Object.entries(budgets).map(([service, budget], i) => {
        const image = images[service === 'seed' ? 'migrate' : service.startsWith('web-') || service === 'api-router' ? 'web' : service];
        const expected = {
            id: (i + 1).toString(16).padStart(64, '0'),
            image,
            service,
            ...budget,
            project,
            owner: project,
            sourceSHA,
            mounts: [],
            tmpfs: {},
            port: null
        };
        return {
            expected,
            observed: {
                Id: expected.id,
                Image: image,
                Config: {
                    Labels: {
                        'com.docker.compose.project': project,
                        'ledgersync.capacity.owner': project,
                        'ledgersync.capacity.source': sourceSHA,
                        'com.docker.compose.service': service
                    }
                },
                State: {
                    Running: !['seed', 'migrate'].includes(service),
                    Paused: false
                },
                HostConfig: {
                    NanoCpus: budget.cpu_millicores * 1e6,
                    Memory: budget.memory_mib * 1024 * 1024,
                    ReadonlyRootfs: true,
                    Privileged: false,
                    CapDrop: ['ALL'],
                    SecurityOpt: ['no-new-privileges:true'],
                    PortBindings: {},
                    Tmpfs: {}
                },
                Mounts: []
            }
        };
    });
    const results = Array.from({
        length: 3000
    }, (_, index) => ({
        index,
        key: 'synthetic-only-one-key',
        kind: index ? 'replay' : 'new',
        status: 'posted',
        latency_ms: 100,
        queue_wait_ms: 0,
        service_ms: 100
    }));
    const steady = {
        name: 'steady',
        observed_seconds: 300,
        offered: 3000,
        completed: 3000,
        dropped: 0,
        generator_max_lag_ms: 0,
        results
    };
    const report = {
        schema: 'ledgersync-http-capacity/v1',
        evidence_kind: 'runtime_capture',
        id: selected.id,
        source_manifest_sha256: sourceSHA,
        machine_sha256: machineSHA,
        images,
        financial_invariants: true,
        unresolved_outcomes: 0,
        counts: {
            posted: 1,
            rejected: 0,
            unresolved: 0
        },
        steady,
        p50_ms: 100,
        p95_ms: 100,
        p99_ms: 100,
        resource_samples: 2,
        observed_steady_seconds: 300,
        new_posted_per_second: 1 / 300,
        technical_failures: 0,
        technical_attempt_failures: 0,
        dropped: 0,
        capacity_qualified: true,
        stop_constraint: null
    };
    report.counts.expected_balances = accountIDs.map(account_id => ({
        account_id,
        ledger_minor: '1000000',
        available_minor: '1000000',
        balance_version: '0'
    }));
    const financial = {
        balances: report.counts.expected_balances,
        accounts: 8,
        balances_checked: 8,
        unbalanced_journals: 0,
        projection_mismatches: 0,
        unresolved_keys: 0,
        duplicate_keys: 0,
        tenant_leaks: 0,
        active_holds: 0,
        posted: 1,
        rejected: 0,
        journals: 1,
        postings: 2,
        outbox_pending: 0,
        outbox_dead: 0
    };
    const resource = {
        status: 'sampled',
        services: enforced.filter(t => !['seed', 'migrate'].includes(t.expected.service)).map(t => ({
            service: t.expected.service,
            id: t.expected.id,
            cpu_percent: 1,
            working_set_bytes: 1024,
            process_rss_bytes_sum: 2048
        })),
        database: {
            pending: 0,
            oldest_seconds: 0,
            dead: 0,
            connections: 8,
            lock_waiters: 0,
            database_bytes: 1048576
        },
        pool_and_policy_metrics: ['ledgersync_database_pool_waits_total 0', 'ledgersync_database_policy_lock_duration_sum 0']
    };
    const files = {
        'definition.json': selected,
        'report.json': report,
        'financial.json': financial,
        'stages.json': [steady],
        'steady-resources.json': [resource, resource],
        'enforcement.json': enforced,
        'cleanup.json': enforced.map(t => ({
            id: t.expected.id,
            removed: true
        }))
    };
    const save = (name, value) => writeFileSync(join(root, name), JSON.stringify(value));
    const seal = () => save('artifacts.json', {
        files: readdirSync(root).filter(n => n !== 'artifacts.json').map(path => {
            const value = readFileSync(join(root, path));
            return {
                path,
                bytes: value.length,
                sha256: sha(value)
            };
        })
    });
    for (const [name, value] of Object.entries(files))
        save(name, value);
    seal();
    return {
        root,
        identity: {
            sourceSHA,
            machineSHA,
            images
        },
        files,
        save,
        seal
    };
}
test('synthetic verifier fixture rejects tampered bytes, unsafe paths, missing captures and changed budgets', () => {
    const f = fixture();
    assert.equal(verifyCapacityCaseEvidence(f.root, f.identity).verified_local_artifacts, true);
    f.save('financial.json', {
        ...f.files['financial.json'],
        projection_mismatches: 1
    });
    assert.throws(() => verifyCapacityCaseEvidence(f.root, f.identity), /hash/);
    f.seal();
    assert.throws(() => verifyCapacityCaseEvidence(f.root, f.identity), /Financial/);
    f.save('financial.json', f.files['financial.json']);
    f.files['enforcement.json'][0].observed.HostConfig.Memory *= 2;
    f.save('enforcement.json', f.files['enforcement.json']);
    f.seal();
    assert.throws(() => verifyCapacityCaseEvidence(f.root, f.identity), /enforcement differs/);
    f.save('artifacts.json', {
        files: [{
                path: '../escape.json',
                bytes: 1,
                sha256: 'a'.repeat(64)
            }]
    });
    assert.throws(() => verifyCapacityCaseEvidence(f.root, f.identity), /unsafe/);
    f.save('artifacts.json', {
        files: []
    });
    assert.throws(() => verifyCapacityCaseEvidence(f.root, f.identity), /incomplete/);
});
test('a forged headline cannot qualify a raw short or dropped interval', () => {
    const f = fixture();
    f.files['report.json'].observed_steady_seconds = 299;
    f.save('report.json', f.files['report.json']);
    f.seal();
    assert.throws(() => verifyCapacityCaseEvidence(f.root, f.identity), /accounting/);
});
test('mount guards require every approved input and reject added temporary locations', () => {
    const f = fixture(), { expected, observed } = f.files['enforcement.json'][0];
    expected.mounts = [{
            source: 'D:/frozen/router.mjs',
            target: '/capacity/router.mjs',
            read_only: true
        }];
    assert.throws(() => assertCapacityTarget(observed, expected), /storage/);
    observed.Mounts = [{
            Type: 'bind',
            Source: '/run/desktop/mnt/host/d/frozen/router.mjs',
            Destination: '/capacity/router.mjs',
            RW: false
        }];
    assert.doesNotThrow(() => assertCapacityTarget(observed, expected));
    observed.Mounts.push({
        Type: 'tmpfs',
        Destination: '/unplanned'
    });
    assert.throws(() => assertCapacityTarget(observed, expected), /storage/);
});
test('case output rejects oversize evidence and keeps space for failure/cleanup records', () => {
    const root = mkdtempSync(join(tmpdir(), 'ledgersync-capacity-output-')), save = createCaseWriter(root, 1024);
    save('stages.json', 'x'.repeat(700));
    save('fixture.sql','-- synthetic');save('collector.json',{});save('runtime.env','SYNTHETIC_ONLY=fixture');
    if(process.platform!=='win32'){assert.equal(statSync(join(root,'fixture.sql')).mode&0o777,0o644);assert.equal(statSync(join(root,'collector.json')).mode&0o777,0o644);assert.equal(statSync(join(root,'runtime.env')).mode&0o777,0o600);}
    assert.throws(() => save('extra.json', 'x'.repeat(100)), /size/);
    save('failure.json', {
        status: 'failed'
    });
    save('cleanup.json', []);
    assert.throws(() => save('../outside', 'value'), /Unsafe/);
    assert.throws(() => save('stages.json', 'changed'));
});
