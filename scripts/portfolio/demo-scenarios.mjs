// Fixed host-owned routes. No browser or CLI input can select a shell command.
export const failureScenarios = Object.freeze([
  ['normal-journey', 'integration', 'TestPortfolioNormalJourney'],
  ['lost-response', 'integration', 'TestPortfolioCommittedResponseLoss'],
  ['duplicate-command', 'integration', 'TestConcurrentSameKeyRequestsResolveToOneMovement'],
  ['changed-intent', 'integration', 'TestTransferIdempotencyRejectsSameKeyWithDifferentFinancialIntent'],
  ['concurrent-debits', 'integration', 'TestCompetingTransfersNeverOverdrawSourceAccount'],
  ['api-failure', 'integration', 'TestPortfolioAPIFailover'],
  ['duplicate-and-reordered-event', 'integration', 'TestBalanceProjectionIsIdempotentAndNeverRegressesVersion'],
  ['stale-worker', 'integration', 'TestOutboxGenerationRejectsPreviousClaimFromSameOwner'],
  ['scheduler-takeover', 'integration', 'TestSchedulerLeaseTakeoverRejectsStaleJobs'],
  ['cache-loss', 'fault', 'TestRedisLossUsesPrimaryAndRebuildsTheBalanceCache'],
  ['dependency-loss', 'fault', 'TestRedisDependencyLossReschedulesWithoutChangingFinancialRecords'],
  ['process-crash', 'integration', 'TestOutboxProcessCrashRecovery'],
  ['reconciliation-during-traffic', 'integration', 'TestReconciliationRecordsOutboxInItsLedgerSnapshot'],
  ['tenant-denial', 'integration', 'TestTenantPermanentPolicyRejectsMalformedContextAndForeignJoin'],
].map(row => Object.freeze(row)));

export const requiredFailureMatrix = Object.freeze({
  'normal-journey': 'normal-journey', 'exact-retry': 'duplicate-command', 'changed-intent-retry': 'changed-intent',
  'lost-committed-response': 'lost-response', 'concurrent-debits': 'concurrent-debits', 'api-failure': 'api-failure',
  'worker-before-publication': 'process-crash', 'worker-after-publication': 'process-crash', 'stale-worker': 'stale-worker',
  'scheduler-failure': 'scheduler-takeover', 'cache-interruption': 'cache-loss', 'reconciliation-during-traffic': 'reconciliation-during-traffic', 'tenant-denial': 'tenant-denial',
});

export function demoPreview() {
  return {
    schema: 'ledgersync-demo-preview/v1', execution: 'none', creates_resources: false, writes_files: false,
    intended_setup: ['Verify source, preserved containers, dependency image identities and disposable target exclusions.', 'Create a new synthetic tmpfs PostgreSQL/Redis project with fixed limits and loopback ports.', 'Apply ordered migrations and restricted workload roles only to that new target.'],
    scenarios: failureScenarios.map(([id, group, test]) => ({ id, group, test })), required_matrix: requiredFailureMatrix,
    fault_points: { 'lost-response': 'Close the owned HTTP connection after Submit commits and before any success response is written; recover with the original key.', 'api-failure': 'Stop the first owned HTTP server after a proven successful request; use the surviving server.', 'process-crash': 'Parent test terminates its own real worker child before and after publication.' },
    intended_recovery: ['Use original keys and full intents.', 'Check database journals, versions, authorization, lease fencing and reconciliation.'],
    intended_cleanup: ['Stop only owned child processes.', 'Verify exact target IDs, images, labels, temporary storage and loopback ports before removal.', 'Retain failed and incomplete evidence; preserve all normal volumes.'],
    limits: ['Preview is a plan, not runtime evidence.', 'The API fault tests use owned HTTP servers and real database application commands; they do not qualify a managed load balancer.'],
  };
}
