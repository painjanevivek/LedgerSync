import { capacityComparisonPlan } from '../capacity-matrix.mjs';
export const capacityPurpose = 'portfolio-http-capacity/v1';
export const tenantIDs = ['00000000-0000-4000-8000-000000009301', '00000000-0000-4000-8000-000000009302'];
export const subjectID = 'synthetic-capacity-operator';
export const accountIDs = Array.from({
    length: 8
}, (_, i) => `10000000-0000-4000-8000-${String(9301 + i).padStart(12, '0')}`);
export const fixedBudgets = capacityComparisonPlan().fixed_services;
export function capacityCaseDefinition(id) {
    const item = capacityComparisonPlan().cases.find(c => c.id === id);
    if (!item)
        throw Error('One exact comparison case ID is required');
    return {
        ...structuredClone(item),
        api_instances: item.allocation.replicas,
        api_budget: item.allocation,
        bff_instances: 2,
        bff_budget: fixedBudgets.web,
        fixed_services: structuredClone(fixedBudgets),
        method: {
            ...capacityComparisonPlan().method,
            warmup_seconds: 10,
            staged_seconds: 10,
            request_timeout_ms: 12000,
            recovery_seconds: 60,
            maximum_operations: 6000
        },
        qualification: 'pending_runtime'
    };
}
export function capacityPreview(ids = capacityComparisonPlan().cases.map(c => c.id)) {
    if (!Array.isArray(ids) || !ids.length || ids.length > 27 || new Set(ids).size !== ids.length)
        throw Error('Unique bounded case IDs required');
    return {
        schema: 'ledgersync-http-capacity-preview/v1',
        execution: 'none',
        docker_started: false,
        files_written: false,
        cases: ids.map(capacityCaseDefinition),
        cleanup: 'Verify exact recorded ID, image, labels, resource limits, ports and temporary mounts before each action; remove only recorded temporary objects.',
        limits: ['No runtime approval or capacity result is produced by preview.', 'Two tenant BFFs share a fixed total CPU/memory budget. Router budget is fixed in every case.', 'Normal containers, volumes, image tags and networks remain excluded.']
    };
}
export function validateImageSet(images) {
    const required = ['api', 'outbox-worker', 'web', 'migrate', 'postgres', 'redis', 'telemetry'];
    if (!images || Object.keys(images).length !== required.length || required.some(k => !/^sha256:[a-f0-9]{64}$/.test(images[k] ?? '') || /^sha256:0+$/.test(images[k])))
        throw Error('Seven immutable local image identities required');
    return images;
}
export function chooseIntent(profile, sequence, runID) {
    if (!['independent-pairs', 'hot-pair', 'multi-tenant'].includes(profile) || !Number.isInteger(sequence) || sequence < 0 || sequence > 6000 || !/^[a-f0-9]{8}$/.test(runID))
        throw Error('Bounded deterministic intent required');
    const replay = sequence > 0 && sequence % 10 === 9, original = replay ? sequence - 1 : sequence;
    const pair = profile === 'hot-pair' ? 0 : profile === 'multi-tenant' ? (original % 2) * 2 : original % 2;
    const tenant = pair === 2 ? 1 : 0, reject = original % 20 === 18;
    const [from, to] = pair === 2 ? [5, 6] : pair === 1 ? [2, 3] : [0, 1];
    const source = reject ? (tenant === 0 ? 4 : 7) : (original % 4 >= 2 ? to : from), destination = original % 4 >= 2 ? from : to;
    const tail = (BigInt('0x' + runID) * 10000n + BigInt(original)).toString(16).padStart(12, '0');
    return {
        sequence,
        original,
        tenant,
        kind: replay ? 'replay' : reject ? 'expected_rejection' : 'new',
        key: `capacity-${runID}-${String(original).padStart(6, '0')}`,
        reference: `93000000-0000-4000-8000-${tail}`,
        body: {
            sourceAccountId: accountIDs[source],
            destinationAccountId: accountIDs[destination],
            amount: {
                currency: 'INR',
                minorUnits: '100'
            }
        }
    };
}
