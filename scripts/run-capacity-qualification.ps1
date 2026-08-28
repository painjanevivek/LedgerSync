[CmdletBinding()]
param(
    [ValidateSet('hot', 'mixed', 'retry')]
    [string]$WorkloadShape = 'mixed',

    [ValidateRange(1, 200)]
    [int]$TransactionsPerSecond = 25,

    [ValidatePattern('^[1-9][0-9]*(s|m)$')]
    [string]$Duration = '5m',

    [string]$OutputPath = '',

    [ValidatePattern('^(compose|ledgersync-acceptance-\d{14}-[0-9a-f]{8})$')]
    [string]$ComposeProject = 'compose',

    [string]$K6Image = 'grafana/k6@sha256:1f40432b1cbe7234e977f96c362c9bc550a2d2b583d014dd8669fe40d3e9e755',

    [switch]$ValidateOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function ConvertTo-CapacityDurationSeconds {
    param([Parameter(Mandatory)][string]$Value)
    if ($Value -notmatch '^([1-9][0-9]*)(s|m)$') { throw 'Duration must be a positive number of seconds or minutes.' }
    $capacityMagnitude = [int64]$Matches[1]
    if ($Matches[2] -eq 'm') { return $capacityMagnitude * 60 }
    return $capacityMagnitude
}

$capacityDurationSeconds = ConvertTo-CapacityDurationSeconds -Value $Duration
$capacityExpectedTransferIterations = [int64]$TransactionsPerSecond * $capacityDurationSeconds
$capacityExpectedControlIterations = if ($WorkloadShape -eq 'mixed') { [int64][math]::Ceiling($capacityDurationSeconds / 60.0) } else { [int64]0 }

if ($ValidateOnly) {
    [ordered]@{
        schema_version = 1
        workload_shape = $WorkloadShape
        offered_tps = $TransactionsPerSecond
        duration = $Duration
        duration_seconds = $capacityDurationSeconds
        expected_transfer_iterations = $capacityExpectedTransferIterations
        expected_control_iterations = $capacityExpectedControlIterations
        k6_image = $K6Image
        executes_load = $false
    } | ConvertTo-Json -Compress
    return
}

$capacityRepository = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$capacityEvidenceRoot = [System.IO.Path]::GetFullPath((Join-Path $capacityRepository '.tmp/capacity-phase1'))
if ([string]::IsNullOrWhiteSpace($OutputPath)) {
    $OutputPath = Join-Path $capacityEvidenceRoot "$WorkloadShape-$TransactionsPerSecond-$Duration.json"
}
$capacityOutput = [System.IO.Path]::GetFullPath($OutputPath)
$capacityPrefix = $capacityEvidenceRoot.TrimEnd('\') + '\'
if (-not $capacityOutput.StartsWith($capacityPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw 'Capacity evidence output must remain inside .tmp/capacity-phase1.'
}
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $capacityOutput) | Out-Null

$capacityComposeFile = Join-Path $capacityRepository 'deploy/compose/docker-compose.yml'
$capacityStateDirectory = [Environment]::GetEnvironmentVariable('LEDGERSYNC_LOCAL_STATE_DIRECTORY')
$capacityEnvironmentFile = if ([string]::IsNullOrWhiteSpace($capacityStateDirectory)) {
    Join-Path $capacityRepository 'data/local-runtime/runtime.env'
} else {
    Join-Path ([IO.Path]::GetFullPath($capacityStateDirectory)) 'runtime.env'
}
if (-not (Test-Path -LiteralPath $capacityEnvironmentFile -PathType Leaf)) {
    throw 'Capacity qualification requires the generated runtime environment file for its exact Compose project.'
}
$capacityPostgresContainer = "$ComposeProject-postgres-1"
$capacityRedisContainer = "$ComposeProject-redis-1"
$capacityContainerNames = @(
    "$ComposeProject-api-1",
    "$ComposeProject-web-1",
    $capacityPostgresContainer,
    $capacityRedisContainer,
    "$ComposeProject-outbox-worker-1"
)
$capacitySource = '10000000-0000-4000-8000-000000000001'
$capacityDestination = '10000000-0000-4000-8000-000000000004'
$capacityTenant = '00000000-0000-4000-8000-000000000001'
$capacityLoadContainer = "ledgersync-capacity-$PID"
$capacityPairs = @(
    '10000000-0000-4000-8000-000000000001>10000000-0000-4000-8000-000000000004',
    '10000000-0000-4000-8000-000000000002>10000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000003>10000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000004>10000000-0000-4000-8000-000000000003'
) -join ';'

function Invoke-CapacityPostgresScalar {
    param([Parameter(Mandatory)][string]$Query)
    $capacityValue = & docker exec $capacityPostgresContainer psql -v ON_ERROR_STOP=1 -U ledgersync -d ledgersync -At -c $Query
    if ($LASTEXITCODE -ne 0) { throw "PostgreSQL evidence query failed: $Query" }
    return ($capacityValue | Select-Object -Last 1).Trim()
}

function Get-CapacityRedisInfo {
    $capacityInfo = @{}
    $capacityLines = & docker exec $capacityRedisContainer redis-cli INFO stats
    if ($LASTEXITCODE -ne 0) { throw 'Redis evidence query failed.' }
    foreach ($capacityLine in $capacityLines) {
        if ($capacityLine -match '^(total_commands_processed|instantaneous_ops_per_sec|total_error_replies):(.+)$') {
            $capacityInfo[$Matches[1]] = $Matches[2].Trim()
        }
    }
    $capacityMemory = & docker exec $capacityRedisContainer redis-cli INFO memory
    if ($LASTEXITCODE -ne 0) { throw 'Redis memory evidence query failed.' }
    foreach ($capacityLine in $capacityMemory) {
        if ($capacityLine -match '^(used_memory|used_memory_peak):(.+)$') {
            $capacityInfo[$Matches[1]] = $Matches[2].Trim()
        }
    }
    return $capacityInfo
}

foreach ($capacityContainer in $capacityContainerNames) {
    $capacityRunning = & docker inspect --format '{{.State.Running}}' $capacityContainer 2>$null
    if ($LASTEXITCODE -ne 0 -or $capacityRunning -ne 'true') {
        throw "Required local container is not running: $capacityContainer"
    }
}

$capacityStartedAt = (Get-Date).ToUniversalTime()
$capacityDatabaseBytesBefore = [int64](Invoke-CapacityPostgresScalar -Query "SELECT pg_database_size('ledgersync')")
$capacityMovementBefore = (Invoke-CapacityPostgresScalar -Query "SELECT (SELECT count(*) FROM transfers WHERE tenant_id='$capacityTenant')||'|'||(SELECT count(*) FROM journal_transactions WHERE tenant_id='$capacityTenant')||'|'||(SELECT count(*) FROM ledger_postings p JOIN journal_transactions j ON j.id=p.journal_transaction_id WHERE j.tenant_id='$capacityTenant')||'|'||(SELECT count(*) FROM idempotency_requests WHERE tenant_id='$capacityTenant' AND operation='transfers.create.v1' AND state='completed' AND transfer_id IS NOT NULL)||'|'||(SELECT count(*) FROM accounts WHERE tenant_id='$capacityTenant' AND external_reference LIKE 'capacity-control-%')").Split('|')
$capacityReconciliationMismatchesBefore = [int64](Invoke-CapacityPostgresScalar -Query "SELECT count(*) FROM reconciliation_mismatches WHERE tenant_id='$capacityTenant'")
$capacityRedisBefore = Get-CapacityRedisInfo
Invoke-CapacityPostgresScalar -Query 'SELECT pg_stat_reset()' | Out-Null

$capacityMonitor = Start-Job -ScriptBlock {
    while ($true) {
        $capacityAt = (Get-Date).ToUniversalTime().ToString('O')
        $capacityDockerRows = & docker stats $using:capacityContainerNames --no-stream --format '{{json .}}'
        foreach ($capacityDockerRow in $capacityDockerRows) {
            $capacityDocker = $capacityDockerRow | ConvertFrom-Json
            [pscustomobject]@{
                Kind = 'docker'; At = $capacityAt; Name = $capacityDocker.Name
                CpuPercent = [double]($capacityDocker.CPUPerc.TrimEnd('%'))
                MemoryPercent = [double]($capacityDocker.MemPerc.TrimEnd('%'))
                Connections = $null; WaitingLocks = $null
            }
        }
        $capacityDatabase = & docker exec $using:capacityPostgresContainer psql -U ledgersync -d ledgersync -At -F '|' -c "SELECT numbackends,(SELECT count(*) FROM pg_stat_activity WHERE datname='ledgersync' AND state='active'),(SELECT count(*) FROM pg_locks WHERE NOT granted) FROM pg_stat_database WHERE datname='ledgersync'"
        if ($LASTEXITCODE -eq 0 -and $capacityDatabase) {
            $capacityFields = ($capacityDatabase | Select-Object -Last 1).Split('|')
            [pscustomobject]@{
                Kind = 'postgres'; At = $capacityAt; Name = $using:capacityPostgresContainer
                CpuPercent = $null; MemoryPercent = $null
                Connections = [int]$capacityFields[0]
                ActiveConnections = [int]$capacityFields[1]
                WaitingLocks = [int]$capacityFields[2]
            }
        }
        Start-Sleep -Seconds 5
    }
}

$capacityContainerOutput = $capacityOutput.Substring($capacityRepository.Length).TrimStart('\').Replace('\', '/')
$capacityK6Arguments = @(
    'run', '--quiet', "--summary-export=/workspace/$capacityContainerOutput",
    '--summary-trend-stats=avg,min,med,p(50),p(90),p(95),p(99),max',
    '-e', 'LEDGERSYNC_PERF_BFF_URL=http://127.0.0.1:3000',
    '-e', 'LEDGERSYNC_PERF_PUBLIC_ORIGIN=http://127.0.0.1:3000',
    '-e', "LEDGERSYNC_PERF_WORKLOAD_SHAPE=$WorkloadShape",
    '-e', "LEDGERSYNC_PERF_SOURCE_ACCOUNT=$capacitySource",
    '-e', "LEDGERSYNC_PERF_DESTINATION_ACCOUNT=$capacityDestination",
    '-e', "LEDGERSYNC_PERF_TPS=$TransactionsPerSecond",
    '-e', "LEDGERSYNC_PERF_DURATION=$Duration"
)
if ($WorkloadShape -eq 'mixed') {
    $capacityK6Arguments += @('-e', "LEDGERSYNC_PERF_ACCOUNT_PAIRS=$capacityPairs")
}
$capacityK6Arguments += 'tests/performance/k6/transfers.js'

try {
    $capacityK6Output = & docker run --network host --name $capacityLoadContainer --rm -v "$($capacityRepository.Replace('\', '/')):/workspace" -w /workspace $K6Image @capacityK6Arguments 2>&1
    $capacityK6ExitCode = $LASTEXITCODE
}
finally {
    Stop-Job -Job $capacityMonitor -ErrorAction SilentlyContinue
    $capacitySamples = @(Receive-Job -Job $capacityMonitor -ErrorAction SilentlyContinue)
    Remove-Job -Job $capacityMonitor -Force -ErrorAction SilentlyContinue
    # A named test-only container makes an interrupted run identifiable and
    # prevents a detached load generator from contaminating a later result.
    & docker rm --force $capacityLoadContainer 2>$null | Out-Null
}
if (-not (Test-Path -LiteralPath $capacityOutput)) {
    $capacityK6Output | Write-Error
    throw "k6 did not produce a qualification summary (exit code $capacityK6ExitCode)."
}

$capacityCompletedAt = (Get-Date).ToUniversalTime()
$capacitySummary = Get-Content -LiteralPath $capacityOutput -Raw | ConvertFrom-Json
$capacityDiagnostics = @(
    $capacityK6Output |
        ForEach-Object {
            $capacityDiagnosticLine = [string]$_
            if ($capacityDiagnosticLine -match 'unexpected_[a-z_]+_status=\d+ code=[a-z0-9_]+') {
                return $Matches[0]
            }
            if ($capacityDiagnosticLine -match 'BFF session unavailable \((\d+)\)') {
                return "bff_session_unavailable_status=$($Matches[1])"
            }
            if ($capacityDiagnosticLine -match '(?i)(connection refused|dial tcp|connection reset|no such host)') {
                return 'bff_session_transport_unavailable'
            }
        } |
        Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) } |
        Select-Object -Unique -First 10
)
$capacityFailedChecks = [ordered]@{}
foreach ($capacityCheck in $capacitySummary.root_group.checks.PSObject.Properties) {
    if ([int]$capacityCheck.Value.fails -gt 0) {
        $capacityFailedChecks[$capacityCheck.Name] = [int]$capacityCheck.Value.fails
    }
}

# Delivery is asynchronous. Give the normal worker a bounded opportunity to
# drain before deciding whether the qualification left unpublished work.
$capacityDrainDeadline = (Get-Date).AddSeconds(60)
do {
    $capacityUndrained = [int](Invoke-CapacityPostgresScalar -Query "SELECT count(*) FROM outbox_events WHERE tenant_id='$capacityTenant' AND published_at IS NULL AND dead_at IS NULL")
    if ($capacityUndrained -eq 0) { break }
    Start-Sleep -Seconds 1
} while ((Get-Date) -lt $capacityDrainDeadline)

$capacityDatabaseStats = (Invoke-CapacityPostgresScalar -Query "SELECT xact_commit||'|'||xact_rollback||'|'||deadlocks||'|'||blks_read||'|'||blks_hit||'|'||temp_files||'|'||temp_bytes FROM pg_stat_database WHERE datname='ledgersync'").Split('|')
$capacityDatabaseBytesAfter = [int64](Invoke-CapacityPostgresScalar -Query "SELECT pg_database_size('ledgersync')")
$capacityOutbox = (Invoke-CapacityPostgresScalar -Query "SELECT count(*) FILTER (WHERE published_at IS NULL AND dead_at IS NULL)||'|'||count(*) FILTER (WHERE dead_at IS NOT NULL)||'|'||COALESCE(EXTRACT(EPOCH FROM now()-min(created_at) FILTER (WHERE published_at IS NULL AND dead_at IS NULL)),0) FROM outbox_events WHERE tenant_id='$capacityTenant'").Split('|')
$capacitySafety = (Invoke-CapacityPostgresScalar -Query "SELECT (SELECT count(*)-count(DISTINCT transfer_id) FROM journal_transactions WHERE tenant_id='$capacityTenant')||'|'||(SELECT count(*) FROM transfers t JOIN accounts d ON d.id=t.debit_account_id JOIN accounts c ON c.id=t.credit_account_id WHERE t.tenant_id='$capacityTenant' AND (t.tenant_id<>d.tenant_id OR t.tenant_id<>c.tenant_id))||'|'||(SELECT count(*) FROM account_balance_projections p JOIN accounts a ON a.id=p.account_id WHERE a.tenant_id='$capacityTenant' AND (p.available_minor<0 OR p.ledger_minor<0))").Split('|')
$capacityMovementAfter = (Invoke-CapacityPostgresScalar -Query "SELECT (SELECT count(*) FROM transfers WHERE tenant_id='$capacityTenant')||'|'||(SELECT count(*) FROM journal_transactions WHERE tenant_id='$capacityTenant')||'|'||(SELECT count(*) FROM ledger_postings p JOIN journal_transactions j ON j.id=p.journal_transaction_id WHERE j.tenant_id='$capacityTenant')||'|'||(SELECT count(*) FROM idempotency_requests WHERE tenant_id='$capacityTenant' AND operation='transfers.create.v1' AND state='completed' AND transfer_id IS NOT NULL)||'|'||(SELECT count(*) FROM accounts WHERE tenant_id='$capacityTenant' AND external_reference LIKE 'capacity-control-%')").Split('|')
$capacityProjectionDrift = [int](Invoke-CapacityPostgresScalar -Query "WITH authoritative AS (SELECT a.id,o.opening_ledger_minor,o.opening_ledger_minor+COALESCE(SUM(CASE WHEN p.direction='credit' THEN p.amount_minor ELSE -p.amount_minor END),0) AS ledger_minor FROM accounts a LEFT JOIN account_opening_balances o ON o.account_id=a.id LEFT JOIN ledger_postings p ON p.account_id=a.id WHERE a.tenant_id='$capacityTenant' GROUP BY a.id,o.opening_ledger_minor) SELECT count(*) FROM authoritative x LEFT JOIN account_balance_projections b ON b.account_id=x.id WHERE x.opening_ledger_minor IS NULL OR b.account_id IS NULL OR b.ledger_minor<>x.ledger_minor OR b.available_minor<>x.ledger_minor")
$capacityReconciliationMismatchesAfter = [int64](Invoke-CapacityPostgresScalar -Query "SELECT count(*) FROM reconciliation_mismatches WHERE tenant_id='$capacityTenant'")
$capacityVelocityDrift = [int](Invoke-CapacityPostgresScalar -Query "WITH expected AS (SELECT tenant_id,SUM(amount_minor) total_minor FROM transfer_velocity_events GROUP BY tenant_id) SELECT count(*) FROM expected e JOIN transfer_velocity_totals t ON t.tenant_id=e.tenant_id AND t.dimension_type='tenant' AND t.dimension_reference=e.tenant_id::text WHERE e.total_minor<>t.total_minor")
$capacityRedisAfter = Get-CapacityRedisInfo

$capacityReconcileOutput = & docker compose --env-file $capacityEnvironmentFile -p $ComposeProject -f $capacityComposeFile run --rm --entrypoint /usr/local/bin/reconcile migrate --run --tenant-id $capacityTenant 2>&1
if ($LASTEXITCODE -ne 0) {
    $capacityReconcileOutput | Write-Error
    throw 'Post-load reconciliation command failed.'
}
$capacityReconciliation = (Invoke-CapacityPostgresScalar -Query "SELECT id::text||'|'||status||'|'||mismatch_count FROM reconciliation_runs WHERE tenant_id='$capacityTenant' ORDER BY completed_at DESC,id DESC LIMIT 1").Split('|')

$capacityDockerSummary = @($capacitySamples | Where-Object Kind -eq 'docker' | Group-Object Name | ForEach-Object {
    $capacityCpu = @($_.Group | ForEach-Object CpuPercent)
    $capacityMemory = @($_.Group | ForEach-Object MemoryPercent)
    [ordered]@{
        name = $_.Name
        samples = $_.Count
        cpu_percent_average = [math]::Round(($capacityCpu | Measure-Object -Average).Average, 3)
        cpu_percent_maximum = [math]::Round(($capacityCpu | Measure-Object -Maximum).Maximum, 3)
        memory_percent_maximum = [math]::Round(($capacityMemory | Measure-Object -Maximum).Maximum, 3)
    }
})
$capacityPostgresSamples = @($capacitySamples | Where-Object Kind -eq 'postgres')
$capacityDroppedIterations = if ($capacitySummary.metrics.PSObject.Properties.Name -contains 'dropped_iterations') { [int]$capacitySummary.metrics.dropped_iterations.count } else { 0 }
$capacityUnexpectedOutcomes = if ($capacitySummary.metrics.PSObject.Properties.Name -contains 'ledgersync_unexpected_outcomes') { [int64]$capacitySummary.metrics.ledgersync_unexpected_outcomes.count } else { 0 }
$capacityTransferIterations = if ($capacitySummary.metrics.PSObject.Properties.Name -contains 'ledgersync_transfer_iterations') { [int64]$capacitySummary.metrics.ledgersync_transfer_iterations.count } else { 0 }
$capacityControlIterations = if ($capacitySummary.metrics.PSObject.Properties.Name -contains 'ledgersync_control_iterations') { [int64]$capacitySummary.metrics.ledgersync_control_iterations.count } else { 0 }
$capacityControlAccounts = if ($capacitySummary.metrics.PSObject.Properties.Name -contains 'ledgersync_control_accounts_created') { [int64]$capacitySummary.metrics.ledgersync_control_accounts_created.count } else { 0 }
$capacityControlReconciliations = if ($capacitySummary.metrics.PSObject.Properties.Name -contains 'ledgersync_control_reconciliations_completed') { [int64]$capacitySummary.metrics.ledgersync_control_reconciliations_completed.count } else { 0 }
$capacityMovementDelta = [ordered]@{
    transfers = [int64]$capacityMovementAfter[0] - [int64]$capacityMovementBefore[0]
    journals = [int64]$capacityMovementAfter[1] - [int64]$capacityMovementBefore[1]
    postings = [int64]$capacityMovementAfter[2] - [int64]$capacityMovementBefore[2]
    completed_transfer_idempotency = [int64]$capacityMovementAfter[3] - [int64]$capacityMovementBefore[3]
    control_accounts = [int64]$capacityMovementAfter[4] - [int64]$capacityMovementBefore[4]
}
$capacityReconciliationMismatchDelta = $capacityReconciliationMismatchesAfter - $capacityReconciliationMismatchesBefore
$capacityControlLatency = [ordered]@{
    account_command_p95_ms = if ($capacitySummary.metrics.PSObject.Properties.Name -contains 'ledgersync_account_command_duration') { $capacitySummary.metrics.ledgersync_account_command_duration.'p(95)' } else { $null }
    reconciliation_command_p95_ms = if ($capacitySummary.metrics.PSObject.Properties.Name -contains 'ledgersync_reconciliation_command_duration') { $capacitySummary.metrics.ledgersync_reconciliation_command_duration.'p(95)' } else { $null }
    diagnostics_p95_ms = if ($capacitySummary.metrics.PSObject.Properties.Name -contains 'ledgersync_diagnostics_duration') { $capacitySummary.metrics.ledgersync_diagnostics_duration.'p(95)' } else { $null }
    events_p95_ms = if ($capacitySummary.metrics.PSObject.Properties.Name -contains 'ledgersync_events_duration') { $capacitySummary.metrics.ledgersync_events_duration.'p(95)' } else { $null }
}
$capacityFailureReasons = [System.Collections.Generic.List[string]]::new()
if ($capacityK6ExitCode -ne 0) { $capacityFailureReasons.Add("k6 exited with code $capacityK6ExitCode because one or more workload thresholds failed") }
if ($capacityUnexpectedOutcomes -ne 0) { $capacityFailureReasons.Add('unexpected workload outcomes were observed') }
if ([double]$capacitySummary.metrics.ledgersync_transfer_duration.'p(95)' -ge 500) { $capacityFailureReasons.Add('transfer p95 was at or above 500 ms') }
if ([double]$capacitySummary.metrics.ledgersync_balance_duration.'p(95)' -ge 200) { $capacityFailureReasons.Add('balance p95 was at or above 200 ms') }
if ($capacityDroppedIterations -ne 0) { $capacityFailureReasons.Add('the requested arrival rate could not be sustained without dropped iterations') }
if ($capacityTransferIterations -ne $capacityExpectedTransferIterations) { $capacityFailureReasons.Add("transfer iteration count was $capacityTransferIterations instead of $capacityExpectedTransferIterations") }
if ($capacityControlIterations -ne $capacityExpectedControlIterations) { $capacityFailureReasons.Add("control iteration count was $capacityControlIterations instead of $capacityExpectedControlIterations") }
if ($WorkloadShape -eq 'mixed' -and $capacityControlAccounts -ne $capacityExpectedControlIterations) { $capacityFailureReasons.Add('one exact-zero account control was not created per low-rate control iteration') }
if ($capacityMovementDelta.transfers -ne $capacityExpectedTransferIterations -or $capacityMovementDelta.journals -ne $capacityExpectedTransferIterations -or $capacityMovementDelta.postings -ne (2 * $capacityExpectedTransferIterations) -or $capacityMovementDelta.completed_transfer_idempotency -ne $capacityExpectedTransferIterations) { $capacityFailureReasons.Add('durable transfer, journal, posting, or idempotency movement counts did not match the requested transfer iterations') }
if ($capacityMovementDelta.control_accounts -ne $capacityExpectedControlIterations) { $capacityFailureReasons.Add('durable low-rate account control count did not match the requested control iterations') }
if ($capacityDurationSeconds -ge 60 -and ($capacityDockerSummary.Count -ne $capacityContainerNames.Count -or $capacityPostgresSamples.Count -eq 0)) { $capacityFailureReasons.Add('resource observation samples were incomplete') }
if ([int64]$capacityDatabaseStats[2] -ne 0) { $capacityFailureReasons.Add('PostgreSQL reported deadlocks during the qualification window') }
if ([int]$capacityOutbox[0] -ne 0 -or [int]$capacityOutbox[1] -ne 0) { $capacityFailureReasons.Add('outbox delivery was not fully drained') }
if ([int64]$capacityRedisAfter.total_error_replies - [int64]$capacityRedisBefore.total_error_replies -ne 0) { $capacityFailureReasons.Add('Redis returned error replies during the evidence window') }
if ([int]$capacitySafety[0] -ne 0 -or [int]$capacitySafety[1] -ne 0 -or [int]$capacitySafety[2] -ne 0 -or $capacityVelocityDrift -ne 0 -or $capacityProjectionDrift -ne 0 -or $capacityReconciliationMismatchDelta -ne 0) { $capacityFailureReasons.Add('a financial safety invariant did not hold') }
if ($capacityReconciliation[1] -ne 'matched' -or [int]$capacityReconciliation[2] -ne 0) { $capacityFailureReasons.Add('post-load reconciliation did not match') }
$capacityEvidence = [ordered]@{
    schema_version = 3
    decision = if ($capacityFailureReasons.Count -eq 0) { 'pass' } else { 'fail' }
    failure_reasons = @($capacityFailureReasons)
    workload = [ordered]@{ shape = $WorkloadShape; offered_tps = $TransactionsPerSecond; duration = $Duration; expected_transfer_iterations = $capacityExpectedTransferIterations; expected_control_iterations = $capacityExpectedControlIterations; k6_image = $K6Image }
    window = [ordered]@{ started_at = $capacityStartedAt.ToString('O'); completed_at = $capacityCompletedAt.ToString('O') }
    k6 = [ordered]@{
        iterations = $capacitySummary.metrics.iterations.count
        transfer_iterations = $capacityTransferIterations
        control_iterations = $capacityControlIterations
        control_accounts_created = $capacityControlAccounts
        control_reconciliations_completed = $capacityControlReconciliations
        achieved_iterations_per_second = $capacitySummary.metrics.iterations.rate
        exit_code = $capacityK6ExitCode
        dropped_iterations = $capacityDroppedIterations
        unexpected_outcomes = $capacityUnexpectedOutcomes
        bounded_diagnostics = $capacityDiagnostics
        failed_checks = $capacityFailedChecks
        http_failure_rate = $capacitySummary.metrics.http_req_failed.value
        simulated_lost_responses = if ($capacitySummary.metrics.PSObject.Properties.Name -contains 'ledgersync_simulated_lost_responses') { $capacitySummary.metrics.ledgersync_simulated_lost_responses.count } else { 0 }
        transfer_ms = [ordered]@{ p50 = $capacitySummary.metrics.ledgersync_transfer_duration.'p(50)'; p95 = $capacitySummary.metrics.ledgersync_transfer_duration.'p(95)'; p99 = $capacitySummary.metrics.ledgersync_transfer_duration.'p(99)'; maximum = $capacitySummary.metrics.ledgersync_transfer_duration.max }
        balance_ms = [ordered]@{ p95 = $capacitySummary.metrics.ledgersync_balance_duration.'p(95)'; p99 = $capacitySummary.metrics.ledgersync_balance_duration.'p(99)'; maximum = $capacitySummary.metrics.ledgersync_balance_duration.max }
        bounded_control_ms = $capacityControlLatency
    }
    postgres = [ordered]@{
        commits = [int64]$capacityDatabaseStats[0]; rollbacks = [int64]$capacityDatabaseStats[1]; deadlocks = [int64]$capacityDatabaseStats[2]
        blocks_read = [int64]$capacityDatabaseStats[3]; blocks_hit = [int64]$capacityDatabaseStats[4]
        temp_files = [int64]$capacityDatabaseStats[5]; temp_bytes = [int64]$capacityDatabaseStats[6]
        maximum_connections = if ($capacityPostgresSamples.Count) { ($capacityPostgresSamples.Connections | Measure-Object -Maximum).Maximum } else { $null }
        maximum_active_connections = if ($capacityPostgresSamples.Count) { ($capacityPostgresSamples.ActiveConnections | Measure-Object -Maximum).Maximum } else { $null }
        maximum_waiting_locks = if ($capacityPostgresSamples.Count) { ($capacityPostgresSamples.WaitingLocks | Measure-Object -Maximum).Maximum } else { $null }
        database_bytes_before = $capacityDatabaseBytesBefore; database_bytes_after = $capacityDatabaseBytesAfter; storage_growth_bytes = $capacityDatabaseBytesAfter - $capacityDatabaseBytesBefore
    }
    redis = [ordered]@{
        before = $capacityRedisBefore; after = $capacityRedisAfter
        commands_processed_delta = [int64]$capacityRedisAfter.total_commands_processed - [int64]$capacityRedisBefore.total_commands_processed
        error_replies_delta = [int64]$capacityRedisAfter.total_error_replies - [int64]$capacityRedisBefore.total_error_replies
        used_memory_peak_bytes = [int64]$capacityRedisAfter.used_memory_peak
    }
    containers = $capacityDockerSummary
    delivery = [ordered]@{ unpublished = [int]$capacityOutbox[0]; dead = [int]$capacityOutbox[1]; oldest_unpublished_seconds = [double]$capacityOutbox[2] }
    safety = [ordered]@{
        duplicate_journal_movements = [int]$capacitySafety[0]; tenant_boundary_violations = [int]$capacitySafety[1]
        negative_balance_projections = [int]$capacitySafety[2]; projection_ledger_drift = $capacityProjectionDrift; velocity_counter_mismatches = $capacityVelocityDrift
        new_reconciliation_mismatches = $capacityReconciliationMismatchDelta
        durable_movement_delta = $capacityMovementDelta
        reconciliation_run_id = $capacityReconciliation[0]; reconciliation_status = $capacityReconciliation[1]; reconciliation_mismatches = [int]$capacityReconciliation[2]
    }
}

$capacityEvidence | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $capacityOutput -Encoding utf8NoBOM
$capacityEvidence | ConvertTo-Json -Depth 4 -Compress
if ($capacityFailureReasons.Count -ne 0) {
    throw "Capacity qualification failed: $($capacityFailureReasons -join '; '). Evidence: $capacityOutput"
}
