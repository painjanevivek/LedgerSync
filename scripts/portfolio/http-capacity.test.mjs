import test from 'node:test';
import assert from 'node:assert/strict';
import { capacityCaseDefinition, capacityPreview, validateImageSet } from './http-capacity/definition.mjs';
import { assertCapacityTarget, validateFinancialSnapshot } from './http-capacity/boundary.mjs';
import { coordinateCapacityCases } from './http-capacity/coordinator.mjs';
import { CapacityWorkload } from './http-capacity/driver.mjs';
import { chooseIntent } from './http-capacity/definition.mjs';
const selected = capacityCaseDefinition('hot-pair/two-api-fixed-total/1');
const digest = 'sha256:' + '1'.repeat(64);
const owner = 'ledgersync-capacity-20261017210000-1234abcd';
const target = {
    id: '2'.repeat(64),
    image: digest,
    project: owner,
    owner,
    service: 'api',
    cpu_millicores: 500,
    memory_mib: 256,
    mounts: [],
    port: null
};
function inspected() {
    return {
        Id: target.id,
        Image: digest,
        Config: {
            Labels: {
                'com.docker.compose.project': owner,
                'ledgersync.capacity.owner': owner,
                'com.docker.compose.service': 'api'
            }
        },
        State: {
            Running: true,
            Paused: false
        },
        HostConfig: {
            NanoCpus: 500000000,
            Memory: 256 * 1024 * 1024,
            ReadonlyRootfs: true,
            Privileged: false,
            CapDrop: ['ALL'],
            SecurityOpt: ['no-new-privileges:true'],
            PortBindings: {}
        },
        Mounts: []
    };
}
test('HTTP comparison preview covers all 27 serial cases without an execution authority', () => {
    const plan = capacityPreview();
    assert.equal(plan.execution, 'none');
    assert.equal(plan.cases.length, 27);
    assert.equal(selected.bff_instances, 2);
    assert.equal(selected.bff_budget.cpu_millicores, 1000);
    assert.equal(selected.api_instances, 2);
    assert.equal(selected.api_budget.cpu_millicores_per_replica, 500);
    assert.throws(() => capacityCaseDefinition('hot-pair/two-api-fixed-total/4'));
    assert.throws(() => validateImageSet({
        api: 'ledgersync-api:latest'
    }));
});
test('owned target guard rejects changed resources, writable binds, remote ports and foreign identities', () => {
    assert.doesNotThrow(() => assertCapacityTarget(inspected(), target));
    for (const mutate of [c => c.Id = '3'.repeat(64), c => c.HostConfig.NanoCpus = 1e9, c => c.HostConfig.Memory = 512 * 1024 * 1024, c => c.Mounts = [{
                Type: 'volume',
                Destination: '/data'
            }], c => c.HostConfig.PortBindings = {
            '8080/tcp': [{
                    HostIp: '0.0.0.0',
                    HostPort: '8080'
                }]
        }, c => c.HostConfig.ReadonlyRootfs = false, c=>c.HostConfig.CapAdd=['SYS_ADMIN'],c=>c.HostConfig.PublishAllPorts=true,c => c.Config.Labels['ledgersync.capacity.owner'] = 'compose']) {
        const c = inspected();
        mutate(c);
        assert.throws(() => assertCapacityTarget(c, target));
    }
    const c=inspected(),expected={...target,networks:[owner+'_private']};assert.throws(()=>assertCapacityTarget(c,expected),/network/);c.NetworkSettings={Networks:{[owner+'_private']:{}}};assert.doesNotThrow(()=>assertCapacityTarget(c,expected));c.NetworkSettings.Networks.normal={};assert.throws(()=>assertCapacityTarget(c,expected),/network/);
});
test('financial acceptance requires complete exact balances, balanced journals and settled keys', () => {
    const good = {
        accounts: 8,
        balances_checked: 8,
        unbalanced_journals: 0,
        projection_mismatches: 0,
        unresolved_keys: 0,
        duplicate_keys: 0,
        tenant_leaks: 0,
        active_holds: 0,
        posted: 4,
        rejected: 1,
        postings: 8,
        journals: 4,
        outbox_pending: 0,
        outbox_dead: 0
    };
    assert.equal(validateFinancialSnapshot(good, {
        posted: 4,
        rejected: 1
    }).verified, true);
    for (const mutate of [s => s.projection_mismatches = 1, s => s.posted = 3, s => s.accounts = 7, s => s.unresolved_keys = 1, s => delete s.tenant_leaks, s => s.postings = 6]) {
        const s = {
            ...good
        };
        mutate(s);
        assert.throws(() => validateFinancialSnapshot(s, {
            posted: 4,
            rejected: 1
        }));
    }
});
test('a stopped-runtime decision invokes no adapter operation', async () => {
    let calls = 0;
    const adapter = new Proxy({}, {
        get() {
            calls++;
            throw Error('No runtime access allowed');
        }
    });
    await assert.rejects(() => coordinateCapacityCases({
        caseIDs: [selected.id],
        runtimeApproved: false,
        adapter
    }), /runtime approval/i);
    assert.equal(calls, 0);
});
test('coordinator retains a failed case and cleans its own target before stopping', async () => {
    const events = [];
    let state;
    const adapter = {
        prepare: async () => ({}),
        startCase: async (_selected, _signal, record) => {
            events.push('start');
            state = record;
        },
        measure: async () => {
            throw Error('financial mismatch');
        },
        recordFailure: async (_state, error) => events.push(error.message),
        cleanup: async (s) => {
            assert.equal(s, state);
            events.push('cleanup');
        },
        finish: async () => events.push('finish')
    };
    await assert.rejects(() => coordinateCapacityCases({
        caseIDs: [selected.id],
        runtimeApproved: true,
        adapter
    }), /financial mismatch/);
    assert.deepEqual(events, ['start', 'financial mismatch', 'cleanup', 'finish']);
});
test('the independent monetary model counts one effect per original key and rejects balanced wrong-money results', () => {
    const workload = new CapacityWorkload(['http://127.0.0.1:45001', 'http://127.0.0.1:45002'], 'hot-pair', '1234abcd');
    const intent = chooseIntent('hot-pair', 8, '1234abcd'), receipt = {
        body: {
            transfer_id: '00000000-0000-4000-8000-000000009399',
            status: 'posted',
            currency: 'INR',
            amount_minor: '100'
        },
        replayed: false
    };
    workload.observe(intent, receipt);
    workload.observe(chooseIntent('hot-pair', 9, '1234abcd'), {
        ...receipt,
        replayed: true
    });
    const counts = workload.counts();
    assert.equal(counts.posted, 1);
    assert.equal(counts.expected_balances[0].ledger_minor, '999900');
    assert.equal(counts.expected_balances[1].ledger_minor, '1000100');
    assert.equal(counts.expected_balances[0].balance_version,'1');assert.equal(counts.expected_balances[1].balance_version,'1');
    assert.throws(() => workload.observe(intent, {
        body: {
            ...receipt.body,
            amount_minor: '101'
        }
    }), /Receipt amount/);
    const snapshot = {
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
        postings: 2,
        journals: 1,
        outbox_pending: 0,
        outbox_dead: 0,
        balances: counts.expected_balances
    };
    assert.doesNotThrow(() => validateFinancialSnapshot(snapshot, counts));
    const changed = structuredClone(snapshot);
    changed.balances[0].ledger_minor = '999899';
    changed.balances[0].available_minor = '999899';
    changed.balances[1].ledger_minor = '1000101';
    changed.balances[1].available_minor = '1000101';
    assert.throws(() => validateFinancialSnapshot(changed, counts), /independent exact-money/);
});
