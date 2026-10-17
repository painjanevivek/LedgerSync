import { isDeepStrictEqual } from 'node:util';
export function assertCapacityTarget(value, expected, { allowStopped = false } = {}) {
    const labels = value?.Config?.Labels, host = value?.HostConfig;
    if (!/^ledgersync-capacity-\d{14}-[a-f0-9]{8}$/.test(expected.project) || !/^[a-f0-9]{64}$/.test(expected.id) || !/^sha256:[a-f0-9]{64}$/.test(expected.image) || expected.owner !== expected.project || value?.Id !== expected.id || value.Image !== expected.image || labels?.['com.docker.compose.project'] !== expected.project || labels?.['ledgersync.capacity.owner'] !== expected.owner || labels?.['com.docker.compose.service'] !== expected.service)
        throw Error('Refuse an unowned capacity target');
    if (!host || host.NanoCpus !== expected.cpu_millicores * 1e6 || host.Memory !== expected.memory_mib * 1024 * 1024 || host.ReadonlyRootfs !== true || host.Privileged !== false || host.CapAdd?.length || host.PublishAllPorts || !host.CapDrop?.includes('ALL') || !host.SecurityOpt?.some(v => v === 'no-new-privileges:true' || v === 'no-new-privileges'))
        throw Error('Capacity resource/security enforcement differs');
    if (!allowStopped && (!value.State?.Running || value.State.Paused))
        throw Error('Owned capacity target is not ready');
    if (expected.sourceSHA && labels['ledgersync.capacity.source'] !== expected.sourceSHA)
        throw Error('Container source binding differs');
    if (expected.tmpfs && !isDeepStrictEqual(host.Tmpfs ?? {}, expected.tmpfs))
        throw Error('Temporary storage limits changed');
    if (expected.networks && !isDeepStrictEqual(Object.keys(value.NetworkSettings?.Networks ?? {}).sort(), [...expected.networks].sort()))
        throw Error('Capacity network boundary differs');
    const canonical = value => {
        const windows = value.replaceAll('\\', '/').replace(/^\/run\/desktop\/mnt\/host\/([a-z])\//i, (_, drive) => `${drive}:/`).replace(/^\/host_mnt\/([a-z])\//i, (_, drive) => `${drive}:/`);
        return /^[a-z]:\//i.test(windows) ? windows.toLowerCase() : windows;
    };
    const matches = (actual, planned) => actual.Type === 'bind' && actual.RW === false && canonical(planned.source) === canonical(actual.Source) && planned.target === actual.Destination;
    if (!Array.isArray(value.Mounts) || value.Mounts.some(m => !(m.Type === 'tmpfs' && Object.hasOwn(expected.tmpfs ?? {}, m.Destination)) && !expected.mounts?.some(e => matches(m, e))) || (expected.mounts ?? []).some(e => !value.Mounts.some(m => matches(m, e))))
        throw Error('Capacity storage is not explicitly temporary or an approved read-only input');
    const bindings = Object.entries(host.PortBindings ?? {}).flatMap(([port, entries]) => (entries ?? []).map(b => ({
        port,
        ...b
    })));
    const wanted = expected.port ? [{
            port: '3000/tcp',
            HostIp: '127.0.0.1',
            HostPort: String(expected.port)
        }] : [];
    if (!isDeepStrictEqual(bindings, wanted))
        throw Error('Capacity port ownership differs');
    return value;
}
export function validateFinancialSnapshot(snapshot, expected) {
    const zero = ['unbalanced_journals', 'projection_mismatches', 'unresolved_keys', 'duplicate_keys', 'tenant_leaks', 'active_holds', 'outbox_pending', 'outbox_dead'];
    if (!snapshot || snapshot.accounts !== 8 || snapshot.balances_checked !== 8 || zero.some(k => snapshot[k] !== 0) || !Number.isInteger(expected.posted) || expected.posted < 0 || !Number.isInteger(expected.rejected) || expected.rejected < 0 || snapshot.posted !== expected.posted || snapshot.rejected !== expected.rejected || snapshot.journals !== expected.posted || snapshot.postings !== expected.posted * 2)
        throw Error('Financial reconciliation or command coverage failed');
    if (expected.expected_balances) {
        const normalize = values => [...values].sort((a, b) => a.account_id.localeCompare(b.account_id));
        if (!Array.isArray(snapshot.balances) || snapshot.balances.length !== 8 || expected.expected_balances.length !== 8 || new Set(expected.expected_balances.map(v => v.account_id)).size !== 8 || !isDeepStrictEqual(normalize(snapshot.balances), normalize(expected.expected_balances)))
            throw Error('Balances differ from the independent exact-money workload model');
    }
    return {
        verified: true,
        ...snapshot
    };
}
export function parseMemory(value) {
    const match = /^([0-9]+(?:\.[0-9]+)?)\s*(B|KiB|MiB|GiB|kB|MB|GB)$/.exec(value.trim());
    if (!match)
        throw Error('Unrecognised container memory observation');
    return Number(match[1]) * ({
        B: 1,
        KiB: 1024,
        MiB: 1024 ** 2,
        GiB: 1024 ** 3,
        kB: 1000,
        MB: 1e6,
        GB: 1e9
    }[match[2]]);
}
