import test from 'node:test';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { readFileSync, writeFileSync, mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { capacityCaseDefinition, capacityPreview, chooseIntent } from './http-capacity/definition.mjs';
import { composeCapacityCase, capacitySeedSQL, collectorConfiguration } from './http-capacity/compose.mjs';
import { coordinateCapacityCases } from './http-capacity/coordinator.mjs';
import { DockerCapacityRuntime } from './http-capacity/runtime.mjs';
import { parseHTTPCapacityOptions } from './run-http-capacity.mjs';
import { sourceIdentity } from './candidate-source.mjs';
const require = createRequire(new URL('../../web/package.json', import.meta.url));
const base = require('yaml').parse(readFileSync(new URL('../../deploy/compose/docker-compose.yml', import.meta.url), 'utf8'));
const images = Object.fromEntries(['api', 'outbox-worker', 'web', 'migrate', 'postgres', 'redis', 'telemetry'].map((s, i) => [s, 'sha256:' + String(i + 1).repeat(64)]));
const project = 'ledgersync-capacity-20261017210000-1234abcd', sourceSHA = 'a'.repeat(64), selected = capacityCaseDefinition('hot-pair/one-api/1');
const generate = selected => composeCapacityCase({
    base,
    selected,
    images,
    project,
    ports: [45001, 45002],
    seedPath: 'D:/private/seed.sql',
    routerPath: 'D:/frozen/router.mjs',
    collectorPath: 'D:/private/collector.json',
    sourceSHA
});
test('all 27 generated topologies enforce the chosen budgets without normal storage or host APIs', () => {
    const original = JSON.stringify(base);
    for (const selected of capacityPreview().cases) {
        const { configuration: c, budgets } = generate(selected);
        assert.deepEqual(c.volumes, {});
        assert.equal(c.networks.private.internal, true);
        for (const [name, s] of Object.entries(c.services)) {
            assert.equal(s.image, images[name === 'seed' ? 'migrate' : name === 'api-peer' ? 'api' : name.startsWith('web-') || name === 'api-router' ? 'web' : name]);
            assert.equal(s.cpus, budgets[name].cpu_millicores / 1000);
            assert.equal(s.mem_limit, `${budgets[name].memory_mib}m`);
            assert.equal(s.read_only, true);
            assert.equal(s.build, undefined);
            assert.equal(s.restart, 'no');
            assert.equal(s.container_name, undefined);
            assert.ok(s.tmpfs.every(m => m.includes('size=')));
            assert.ok((s.volumes ?? []).every(m => m.type === 'bind' && m.read_only));
            assert.ok((s.ports ?? []).every(p => name.startsWith('web-') && p.host_ip === '127.0.0.1'));
            for (const dependency of Object.keys(s.depends_on ?? {}))
                assert.ok(c.services[dependency], `${name} requires ${dependency}`);
        }
        assert.equal(Object.keys(c.services).filter(n => n === 'api' || n === 'api-peer').length, selected.api_instances);
        assert.equal(c.services['web-0'].environment.LEDGERSYNC_PRIVATE_API_URL, 'http://api-router:8080');
        assert.notEqual(c.services['web-0'].environment.LEDGERSYNC_LOCAL_TENANT_ID, c.services['web-1'].environment.LEDGERSYNC_LOCAL_TENANT_ID);
    }
    assert.equal(JSON.stringify(base), original);
    const fixture = capacitySeedSQL();
    assert.equal((fixture.match(/INSERT INTO accounts\(/g) ?? []).length, 8);
    assert.equal((fixture.match(/INSERT INTO tenants\(/g) ?? []).length, 2);
    const telemetry = collectorConfiguration();
    assert.deepEqual(Object.keys(telemetry.exporters).sort(), ['debug', 'prometheus']);
    assert.equal(telemetry.service.pipelines.traces.processors.includes('transform/privacy'), true);
});
test('intent replay preserves full tenant-scoped identity and does not create another financial command', () => {
    for (const profile of ['independent-pairs', 'hot-pair', 'multi-tenant'])
        for (let seq = 9; seq < 6000; seq += 10) {
            const original = chooseIntent(profile, seq - 1, 'ffffffff'), retry = chooseIntent(profile, seq, 'ffffffff');
            assert.equal(retry.key, original.key);
            assert.equal(retry.reference, original.reference);
            assert.deepEqual(retry.body, original.body);
            assert.equal(retry.tenant, original.tenant);
            assert.match(retry.reference, /^[a-f0-9-]{36}$/);
        }
});
test('partial startup and cancellation retain ownership and always finish cleanup', async () => {
    const events = [], controller = new AbortController();
    const adapter = {
        prepare: async () => events.push('prepare'),
        startCase: async (_c, _signal, state) => {
            state.owned = 'synthetic-owned-id';
            controller.abort();
            throw Error('startup cancelled');
        },
        measure: async () => assert.fail('Cannot measure partial startup'),
        recordFailure: async (state) => {
            assert.equal(state.owned, 'synthetic-owned-id');
            events.push('failure');
        },
        cleanup: async (state) => {
            assert.equal(state.owned, 'synthetic-owned-id');
            events.push('cleanup');
        },
        finish: async (r) => {
            assert.equal(r.failed, true);
            events.push('finish');
        }
    };
    await assert.rejects(() => coordinateCapacityCases({
        caseIDs: [selected.id],
        runtimeApproved: true,
        adapter,
        signal: controller.signal
    }), /startup cancelled/);
    assert.deepEqual(events, ['prepare', 'failure', 'cleanup', 'finish']);
});
test('serial repeats retain unfavourable performance; cleanup errors cannot erase the original fault', async () => {
    let active = 0, peak = 0, started = 0;
    const adapter = {
        prepare: async () => {
        },
        startCase: async () => {
            active++;
            started++;
            peak = Math.max(peak, active);
        },
        measure: async () => ({
            capacity_qualified: false,
            stop_constraint: 'command_p95'
        }),
        recordFailure: async () => {
        },
        cleanup: async () => {
            active--;
        },
        finish: async () => {
        }
    };
    assert.equal((await coordinateCapacityCases({
        caseIDs: [selected.id, 'hot-pair/one-api/2'],
        runtimeApproved: true,
        adapter
    })).length, 2);
    assert.equal(started, 2);
    assert.equal(peak, 1);
    adapter.measure = async () => {
        throw Error('wrong balance');
    };
    adapter.cleanup = async () => {
        throw Error('cleanup blocked');
    };
    await assert.rejects(() => coordinateCapacityCases({
        caseIDs: [selected.id],
        runtimeApproved: true,
        adapter
    }), error => error instanceof AggregateError && error.errors[0].message === 'wrong balance' && error.errors[1].message === 'cleanup blocked');
});
test('repeated partial discovery retains known targets and never calls Docker in this fixture', () => {
    const runtime = new DockerCapacityRuntime();
    runtime.candidate = {
        manifest_sha256: sourceSHA
    };
    let saves = 0;
    const state = {
        project,
        targets: [],
        config: {
            services: {
                api: {
                    image: images.api,
                    tmpfs: [],
                    networks: ['private']
                }
            }
        },
        budgets: {
            api: {
                cpu_millicores: 1000,
                memory_mib: 512
            }
        },
        save: () => saves++
    };
    runtime.command = args => {
        assert.equal(args[0], 'ps');
        return '1'.repeat(64);
    };
    runtime.inspect = id => ({
        Id: id,
        Config: {
            Labels: {
                'com.docker.compose.service': 'api'
            }
        }
    });
    runtime.verify = () => {
    };
    runtime.discoverTargets(state);
    runtime.discoverTargets(state);
    assert.equal(state.targets.length, 1);
    assert.equal(saves, 2);
    assert.equal(state.discovery, 2);
});
test('the concrete adapter checks authority before any Docker or source operation', async () => {
    const runtime = new DockerCapacityRuntime();
    let calls = 0;
    runtime.command = () => {
        calls++;
        throw Error('Unexpected Docker access');
    };
    await assert.rejects(() => runtime.prepare(), /approval/);
    assert.equal(calls, 0);
    const state={caseSignal:new AbortController().signal};runtime.sql=()=>({pending:0,dead:0});assert.equal((await runtime.waitDrained(state)).pending,0);runtime.sql=()=>({pending:0,dead:1});await assert.rejects(()=>runtime.waitDrained(state),/Dead publication/);
});
test('asynchronous sampling yields to the caller and retains unavailable samples', async () => {
    const runtime = new DockerCapacityRuntime();
    let release, calls = 0;
    runtime.commandAsync = async () => {
        calls++;
        await new Promise(resolve => release = resolve);
        throw Error('Synthetic unavailable resource observation');
    };
    const pending = runtime.sample({
        targets: []
    });
    await Promise.resolve();
    assert.equal(calls, 1);
    release();
    await assert.rejects(() => pending, /unavailable/);
    const state = {
        stoppedSampling: false,
        samples: []
    };
    runtime.sample = async () => {
        state.stoppedSampling = true;
        throw Error('No measurements');
    };
    await runtime.sampleLoop(state);
    assert.equal(state.samples[0].status, 'unavailable');
});
test('cleanup refuses a changed target, continues with other exact owned IDs, and records both results', async () => {
    const runtime = new DockerCapacityRuntime(), root = mkdtempSync(join(tmpdir(), 'ledgersync-capacity-cleanup-')), removed = [];
    const state = {
        root,
        project,
        targets: [{
                id: 'changed',
                service: 'api'
            }, {
                id: 'owned',
                service: 'redis'
            }],
        samples: [],
        secrets: [],
        save: (name, value) => writeFileSync(join(root, name), JSON.stringify(value))
    };
    runtime.verify = (_state, target) => {
        if (target.id === 'changed')
            throw Error('Source identity changed');
        return {
            State: {
                Paused: false
            }
        };
    };
    runtime.command = args => {
        if (args[0] === 'rm') {
            assert.equal(args[2], 'owned');
            removed.push(args[2]);
            return '';
        }
        if (args[0] === 'logs')
            throw Error('Synthetic log capture unavailable');
        if (args[0] === 'network' && args[1] === 'ls')
            return '';
        assert.fail('Unexpected command ' + args[0]);
    };
    await assert.rejects(() => runtime.cleanup(state), /incomplete/);
    assert.deepEqual(removed, ['owned']);
    const records = JSON.parse(readFileSync(join(root, 'cleanup.json'), 'utf8'));
    assert.equal(records[0].removed, false);
    assert.equal(records[1].removed, true);
});
test('a failed raw-artifact verification is recorded as failed even after preservation passes', async () => {
    const runtime = new DockerCapacityRuntime();
    runtime.root = mkdtempSync(join(tmpdir(), 'ledgersync-capacity-finish-'));
    runtime.candidate = {
        manifest_sha256: sourceSHA
    };
    runtime.before = sourceIdentity();
    runtime.normalBefore = [];
    runtime.normal = () => [];
    runtime.machineSHA = 'b'.repeat(64);
    runtime.images = images;
    await assert.rejects(() => runtime.finish({
        reports: [{
                id: selected.id,
                capacity_qualified: true
            }],
        failed: false
    }));
    const completion = JSON.parse(readFileSync(join(runtime.root, 'completion.json'), 'utf8'));
    assert.equal(completion.failed, true);
    assert.equal(completion.preserved, true);
    assert.equal(completion.verified_case_artifacts, 0);
});
test('CLI preview is read-only; execution requires a separate authority before runtime import', () => {
    const env = {
        ...process.env
    };
    delete env.LEDGERSYNC_CAPACITY_RUNTIME_APPROVED;
    const preview = spawnSync(process.execPath, ['scripts/portfolio/run-http-capacity.mjs', '--preview', '--case=' + selected.id], {
        encoding: 'utf8',
        env,
        windowsHide: true
    });
    assert.equal(preview.status, 0, preview.stderr);
    assert.equal(JSON.parse(preview.stdout).docker_started, false);
    const execute = spawnSync(process.execPath, ['scripts/portfolio/run-http-capacity.mjs', '--execute', '--case=' + selected.id], {
        encoding: 'utf8',
        env,
        windowsHide: true
    });
    assert.equal(execute.status, 1);
    assert.match(execute.stderr, /blocked or failed/);
    for (const args of [['--execute'], ['--all', '--case=' + selected.id], ['--execute', '--preview'], ['--unknown'], ['--case=' + selected.id, '--case=' + selected.id]])
        assert.throws(() => parseHTTPCapacityOptions(args));
});
