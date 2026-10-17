import { capacityPreview } from './definition.mjs';
// Runtime adapters are constructed only after the caller passes the explicit
// execution gate. Importing or previewing this module starts nothing.
export async function coordinateCapacityCases({ caseIDs, runtimeApproved = false, adapter, signal }) {
    if (runtimeApproved !== true)
        throw Error('Explicit later runtime approval is required; Docker remains stopped');
    const cases = capacityPreview(caseIDs).cases;
    const reports = [];
    let failed;
    try {
        await adapter.prepare(signal);
        for (const selected of cases) {
            signal?.throwIfAborted();
            // Allocate an ownership record before startup so partial startup is
            // discoverable and cleanup can verify exact objects after an error.
            const state = {
                selected
            };
            let caseFailure;
            try {
                await adapter.startCase(selected, signal, state);
                const report = await adapter.measure(state, signal);
                reports.push(report);
            }
            catch (error) {
                caseFailure = error;
                try {
                    await adapter.recordFailure(state, error);
                }
                catch (recordError) {
                    caseFailure = new AggregateError([error, recordError], 'Case failed; failure recording also failed');
                }
            }
            finally {
                try {
                    await adapter.cleanup(state);
                }
                catch (cleanupError) {
                    caseFailure = caseFailure ? new AggregateError([caseFailure, cleanupError], 'Case failed; owned cleanup also failed') : cleanupError;
                }
            }
            if (caseFailure)
                throw caseFailure;
        }
    }
    catch (error) {
        failed = error;
    }
    finally {
        try {
            await adapter.finish({
                reports,
                failed: !!failed
            });
        }
        catch (finishError) {
            failed = failed ? new AggregateError([failed, finishError], 'Capacity run failed; preservation check also failed') : finishError;
        }
    }
    if (failed)
        throw failed;
    return reports;
}
