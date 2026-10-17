import { pathToFileURL } from 'node:url';
import { capacityPreview } from './http-capacity/definition.mjs';
import { coordinateCapacityCases } from './http-capacity/coordinator.mjs';
export function parseHTTPCapacityOptions(argv) {
    let mode = 'preview', all = false;
    const ids = [], seen = new Set();
    for (const arg of argv) {
        const key = arg.split('=')[0];
        if (seen.has(key) && key !== '--case')
            throw Error('Repeated HTTP capacity option');
        seen.add(key);
        if (arg === '--preview')
            mode = 'preview';
        else if (arg === '--execute')
            mode = 'execute';
        else if (arg === '--all')
            all = true;
        else if (arg.startsWith('--case='))
            ids.push(arg.slice(7));
        else
            throw Error('Unknown HTTP capacity option');
    }
    if (seen.has('--preview') && seen.has('--execute') || all && ids.length || mode === 'execute' && !all && !ids.length)
        throw Error('Choose preview or explicit execution with cases or --all');
    const preview = capacityPreview(ids.length ? ids : undefined);
    return {
        mode,
        caseIDs: preview.cases.map(c => c.id),
        preview
    };
}
export async function main(argv = process.argv.slice(2)) {
    const options = parseHTTPCapacityOptions(argv);
    if (options.mode === 'preview') {
        console.log(JSON.stringify(options.preview, null, 2));
        return;
    }
    if (process.env.LEDGERSYNC_CAPACITY_RUNTIME_APPROVED !== 'true')
        throw Error('Docker execution remains stopped: explicit later runtime approval is required');
    const { DockerCapacityRuntime } = await import('./http-capacity/runtime.mjs');
    const controller = new AbortController(), stop = () => controller.abort();
    const timer = setTimeout(stop, 6 * 60 * 60 * 1000);
    process.once('SIGINT', stop);
    process.once('SIGTERM', stop);
    try {
        const adapter = new DockerCapacityRuntime({
            signal: controller.signal,
            runtimeApproved: true
        });
        const reports = await coordinateCapacityCases({
            caseIDs: options.caseIDs,
            runtimeApproved: true,
            adapter,
            signal: controller.signal
        });
        console.log(JSON.stringify({
            cases: reports.length,
            qualified: reports.every(r => r.capacity_qualified),
            execution: 'local_only'
        }));
        if (reports.some(r => !r.capacity_qualified))
            process.exitCode = 1;
    }
    finally {
        clearTimeout(timer);
        process.removeListener('SIGINT', stop);
        process.removeListener('SIGTERM', stop);
    }
}
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href)
    main().catch(() => {
        console.error('HTTP capacity execution is blocked or failed. Preserve private evidence; no capacity pass is claimed.');
        process.exitCode = 1;
    });
