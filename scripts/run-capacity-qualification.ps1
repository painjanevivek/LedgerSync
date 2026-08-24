[CmdletBinding()]
param(
    [ValidateSet('hot', 'mixed', 'retry')]
    [string]$WorkloadShape = 'mixed',

    [ValidateRange(1, 200)]
    [int]$TransactionsPerSecond = 50,

    [ValidatePattern('^[1-9][0-9]*(s|m)$')]
    [string]$Duration = '5m',

    [string]$OutputPath = '',

    [ValidatePattern('^(compose|ledgersync-acceptance-\d{14}-[0-9a-f]{8})$')]
    [string]$ComposeProject = 'compose',

    [string]$K6Image = 'grafana/k6@sha256:1f40432b1cbe7234e977f96c362c9bc550a2d2b583d014dd8669fe40d3e9e755'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

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
    '-e', 'LEDGERSYNC_PERF_BFF_URL=http://host.docker.internal:3000',
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
    $capacityK6Output = & docker run --name $capacityLoadContainer --rm -v "$($capacityRepository.Replace('\', '/')):/workspace" -w /workspace $K6Image @capacityK6Arguments 2>&1
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
$capacityDiagnostics = @($capacityK6Output | Where-Object { [string]$_ -match 'unexpected_[a-z_]+_status=\d+ code=[a-z0-9_]+' } | Select-Object -First 10)
$capacityFailedChecks = [ordered]@{}
foreach ($capacityCheck in $capacitySummary.root_group.checks.PSObject.Properties) {
    if ([int]$capacityCheck.Value.fails -gt 0) {
        $capacityFailedChecks[$capacityCheck.Name] = [int]$capacityCheck.Value.fails
    }
}
$capacityDatabaseStats = (Invoke-CapacityPostgresScalar -Query "SELECT xact_commit||'|'||xact_rollback||'|'||deadlocks||'|'||blks_read||'|'||blks_hit||'|'||temp_files||'|'||temp_bytes FROM pg_stat_database WHERE datname='ledgersync'").Split('|')
$capacityDatabaseBytesAfter = [int64](Invoke-CapacityPostgresScalar -Query "SELECT pg_database_size('ledgersync')")
$capacityOutbox = (Invoke-CapacityPostgresScalar -Query "SELECT count(*) FILTER (WHERE published_at IS NULL AND dead_at IS NULL)||'|'||count(*) FILTER (WHERE dead_at IS NOT NULL)||'|'||COALESCE(EXTRACT(EPOCH FROM now()-min(created_at) FILTER (WHERE published_at IS NULL AND dead_at IS NULL)),0) FROM outbox_events").Split('|')
$capacitySafety = (Invoke-CapacityPostgresScalar -Query "SELECT (SELECT count(*)-count(DISTINCT transfer_id) FROM journal_transactions)||'|'||(SELECT count(*) FROM transfers t JOIN accounts d ON d.id=t.debit_account_id JOIN accounts c ON c.id=t.credit_account_id WHERE t.tenant_id<>d.tenant_id OR t.tenant_id<>c.tenant_id)||'|'||(SELECT count(*) FROM account_balance_projections WHERE available_minor<0 OR ledger_minor<0)").Split('|')
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
$capacityFailureReasons = [System.Collections.Generic.List[string]]::new()
if ($capacityK6ExitCode -ne 0) { $capacityFailureReasons.Add("k6 exited with code $capacityK6ExitCode because one or more workload thresholds failed") }
if ([int]$capacitySummary.metrics.ledgersync_unexpected_outcomes.count -ne 0) { $capacityFailureReasons.Add('unexpected workload outcomes were observed') }
if ([double]$capacitySummary.metrics.ledgersync_transfer_duration.'p(95)' -ge 500) { $capacityFailureReasons.Add('transfer p95 was at or above 500 ms') }
if ([double]$capacitySummary.metrics.ledgersync_balance_duration.'p(95)' -ge 200) { $capacityFailureReasons.Add('balance p95 was at or above 200 ms') }
if ($capacityDroppedIterations -ne 0) { $capacityFailureReasons.Add('the requested arrival rate could not be sustained without dropped iterations') }
if ([int]$capacityOutbox[0] -ne 0 -or [int]$capacityOutbox[1] -ne 0) { $capacityFailureReasons.Add('outbox delivery was not fully drained') }
if ([int64]$capacityRedisAfter.total_error_replies - [int64]$capacityRedisBefore.total_error_replies -ne 0) { $capacityFailureReasons.Add('Redis returned error replies during the evidence window') }
if ([int]$capacitySafety[0] -ne 0 -or [int]$capacitySafety[1] -ne 0 -or [int]$capacitySafety[2] -ne 0 -or $capacityVelocityDrift -ne 0) { $capacityFailureReasons.Add('a financial safety invariant did not hold') }
if ($capacityReconciliation[1] -ne 'matched' -or [int]$capacityReconciliation[2] -ne 0) { $capacityFailureReasons.Add('post-load reconciliation did not match') }
$capacityEvidence = [ordered]@{
    schema_version = 2
    decision = if ($capacityFailureReasons.Count -eq 0) { 'pass' } else { 'fail' }
    failure_reasons = @($capacityFailureReasons)
    workload = [ordered]@{ shape = $WorkloadShape; offered_tps = $TransactionsPerSecond; duration = $Duration; k6_image = $K6Image }
    window = [ordered]@{ started_at = $capacityStartedAt.ToString('O'); completed_at = $capacityCompletedAt.ToString('O') }
    k6 = [ordered]@{
        iterations = $capacitySummary.metrics.iterations.count
        achieved_iterations_per_second = $capacitySummary.metrics.iterations.rate
        exit_code = $capacityK6ExitCode
        dropped_iterations = $capacityDroppedIterations
        unexpected_outcomes = $capacitySummary.metrics.ledgersync_unexpected_outcomes.count
        bounded_diagnostics = $capacityDiagnostics
        failed_checks = $capacityFailedChecks
        http_failure_rate = $capacitySummary.metrics.http_req_failed.value
        simulated_lost_responses = if ($capacitySummary.metrics.PSObject.Properties.Name -contains 'ledgersync_simulated_lost_responses') { $capacitySummary.metrics.ledgersync_simulated_lost_responses.count } else { 0 }
        transfer_ms = [ordered]@{ p50 = $capacitySummary.metrics.ledgersync_transfer_duration.'p(50)'; p95 = $capacitySummary.metrics.ledgersync_transfer_duration.'p(95)'; p99 = $capacitySummary.metrics.ledgersync_transfer_duration.'p(99)'; maximum = $capacitySummary.metrics.ledgersync_transfer_duration.max }
        balance_ms = [ordered]@{ p95 = $capacitySummary.metrics.ledgersync_balance_duration.'p(95)'; p99 = $capacitySummary.metrics.ledgersync_balance_duration.'p(99)'; maximum = $capacitySummary.metrics.ledgersync_balance_duration.max }
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
        negative_balance_projections = [int]$capacitySafety[2]; velocity_counter_mismatches = $capacityVelocityDrift
        reconciliation_run_id = $capacityReconciliation[0]; reconciliation_status = $capacityReconciliation[1]; reconciliation_mismatches = [int]$capacityReconciliation[2]
    }
}

$capacityEvidence | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $capacityOutput -Encoding utf8NoBOM
$capacityEvidence | ConvertTo-Json -Depth 4 -Compress
if ($capacityFailureReasons.Count -ne 0) {
    throw "Capacity qualification failed: $($capacityFailureReasons -join '; '). Evidence: $capacityOutput"
}
