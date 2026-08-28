[CmdletBinding()]
param(
    [string]$TenantId = "00000000-0000-4000-8000-000000000001"
)

. (Join-Path $PSScriptRoot "local-runtime-common.ps1")

$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true
$recoveryRequired = $false

function Invoke-LedgerSyncCacheRebuild {
    param([Parameter(Mandatory = $true)][string]$TargetTenantId)

    $output = @(Invoke-LedgerSyncCompose -ComposeArguments @(
        "run", "--rm", "--no-deps", "--entrypoint", "/usr/local/bin/reconcile", "api",
        "--run", "--rebuild-cache", "--tenant-id", $TargetTenantId
    ) -CaptureOutput)
    if (($output -join ' ') -notmatch 'status=matched' -or ($output -join ' ') -notmatch 'mismatch_count=0') {
        throw "Cache rebuild did not produce a matched, zero-mismatch reconciliation."
    }
    return $output
}

function Assert-LedgerSyncDependencyUnavailableResponse {
    param([Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session)

    $response = Invoke-WebRequest -UseBasicParsing -SkipHttpErrorCheck `
        -WebSession $Session -TimeoutSec 8 `
        -Uri "$script:LedgerSyncWebUrl/api/me/accounts?limit=1"
    if ([int]$response.StatusCode -ne 503) {
        throw "PostgreSQL outage returned HTTP $([int]$response.StatusCode); expected sanitized temporary unavailability (503)."
    }
    $payload = $response.Content | ConvertFrom-Json
    if ($payload.error.code -cne "account_directory_unavailable") {
        throw "PostgreSQL outage did not return the bounded account-directory unavailable code."
    }
}

function Assert-LedgerSyncRedisDiagnosticDegradation {
    param([Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session)

    $response = Invoke-WebRequest -UseBasicParsing -SkipHttpErrorCheck `
        -WebSession $Session -TimeoutSec 8 `
        -Uri "$script:LedgerSyncWebUrl/api/local/diagnostics"
    if ([int]$response.StatusCode -ne 200) {
        throw "Redis outage diagnostics returned HTTP $([int]$response.StatusCode); expected a truthful partial response."
    }
    $payload = $response.Content | ConvertFrom-Json
    if ($payload.overall_state -cne "degraded" -or
        $payload.financial_authority.postgres.state -cne "reachable" -or
        $payload.delivery_cache.redis.state -cne "unavailable" -or
        $payload.delivery_cache.redis.label -cne "disposable_cache") {
        throw "Redis outage diagnostics did not separate financial authority from disposable-cache degradation."
    }
    foreach ($forbidden in @("password", "token", "container", "docker", "dsn", "connection_string", "payload")) {
        if ($response.Content -cmatch $forbidden) {
            throw "Redis outage diagnostics exposed forbidden infrastructure or payload vocabulary: $forbidden"
        }
    }
}

try {
    Assert-LedgerSyncDockerAvailable
    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy
    Invoke-LedgerSyncWebSmoke
    $baseline = Get-LedgerSyncFinancialFingerprint
    $operationsSession = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
    Initialize-LedgerSyncLocalWebSession -Session $operationsSession -TimeoutSeconds 8

    $redisContainerOutput = @(Invoke-LedgerSyncCompose -ComposeArguments @("ps", "-q", "redis") -CaptureOutput)
    $redisContainer = ([string]($redisContainerOutput | Select-Object -Last 1)).Trim()
    if ($redisContainer -cnotmatch '^[0-9a-f]{12,64}$') {
        throw "Could not resolve the exact Redis container."
    }

    $recoveryRequired = $true
    Invoke-LedgerSyncCompose -ComposeArguments @("stop", "outbox-worker") | Out-Null
    & docker exec $redisContainer redis-cli FLUSHALL | Out-Null
    if ($LASTEXITCODE -ne 0 -or [int64]((& docker exec $redisContainer redis-cli DBSIZE).Trim()) -ne 0) {
        throw "Redis flush fault was not applied as expected."
    }
    $cacheRebuildOutput = Invoke-LedgerSyncCacheRebuild -TargetTenantId $TenantId
    Invoke-LedgerSyncCompose -ComposeArguments @("up", "-d", "--wait", "outbox-worker") | Out-Null
    Invoke-LedgerSyncWebSmoke
    $redisKeyCount = [int64]((& docker exec $redisContainer redis-cli DBSIZE).Trim())
    if ($redisKeyCount -lt 1) {
        throw "Redis did not converge from PostgreSQL after cache loss."
    }
    Compare-LedgerSyncFinancialFingerprint -Before $baseline -After (Get-LedgerSyncFinancialFingerprint)
    Write-Output "REDIS_FLUSH_REBUILD=PASS keys=$redisKeyCount"

    Invoke-LedgerSyncCompose -ComposeArguments @("stop", "redis") | Out-Null
    Assert-LedgerSyncRedisDiagnosticDegradation -Session $operationsSession
    Invoke-LedgerSyncWebSmoke -TimeoutSeconds 8
    Write-Output "REDIS_UNAVAILABLE_DIAGNOSTICS_AND_PRIMARY_FALLBACK=PASS"
    Invoke-LedgerSyncCompose -ComposeArguments @("up", "-d", "--wait", "redis", "outbox-worker", "api", "web") | Out-Null

    Invoke-LedgerSyncCompose -ComposeArguments @("restart", "outbox-worker") | Out-Null
    Invoke-LedgerSyncCompose -ComposeArguments @("up", "-d", "--wait", "outbox-worker") | Out-Null
    Write-Output "OUTBOX_WORKER_RESTART=PASS"

    Invoke-LedgerSyncCompose -ComposeArguments @("restart", "api", "web") | Out-Null
    Invoke-LedgerSyncCompose -ComposeArguments @("up", "-d", "--wait", "api", "web") | Out-Null
    Invoke-LedgerSyncWebSmoke
    Write-Output "STATELESS_SERVICE_RESTART=PASS"

    $session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
    Initialize-LedgerSyncLocalWebSession -Session $session -TimeoutSeconds 8
    Invoke-LedgerSyncCompose -ComposeArguments @("stop", "postgres") | Out-Null
    Assert-LedgerSyncDependencyUnavailableResponse -Session $session
    Write-Output "POSTGRES_UNAVAILABLE_503=PASS"
    Invoke-LedgerSyncCompose -ComposeArguments @("up", "-d", "--wait") | Out-Null

    Invoke-LedgerSyncCompose -ComposeArguments @("stop", "web", "api", "outbox-worker", "redis", "postgres") | Out-Null
    Invoke-LedgerSyncCompose -ComposeArguments @("up", "-d", "--wait") | Out-Null
    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy
    Invoke-LedgerSyncWebSmoke
    Write-Output "DEPENDENCY_ORDER_RECOVERY=PASS"

    $finalRebuildOutput = Invoke-LedgerSyncCacheRebuild -TargetTenantId $TenantId
    Compare-LedgerSyncFinancialFingerprint -Before $baseline -After (Get-LedgerSyncFinancialFingerprint)
    $summary = Get-LedgerSyncOperationalSummary
    if ([int64]$summary.outbox_dead -ne 0 -or [int64]$summary.reconciliation_mismatches -ne 0 -or $summary.reconciliation_status -notin @("completed", "matched", "passed")) {
        throw "Recovery ended with unhealthy operational evidence."
    }

    $recoveryRequired = $false
    Write-Output "FAULT_RECOVERY_SUITE=PASS"
    Write-Output "AUTHORITATIVE_STATE_UNCHANGED=PASS"
    Write-Output "OUTBOX=pending:$($summary.outbox_pending),dead:$($summary.outbox_dead)"
    Write-Output "RECONCILIATION=status:$($summary.reconciliation_status),mismatches:$($summary.reconciliation_mismatches)"
    Write-Output "CACHE_REBUILD=$($cacheRebuildOutput -join ' ')"
    Write-Output "FINAL_REBUILD=$($finalRebuildOutput -join ' ')"
}
catch {
    Write-Error $_
    exit 1
}
finally {
    if ($recoveryRequired) {
        $PSNativeCommandUseErrorActionPreference = $false
        Invoke-LedgerSyncCompose -ComposeArguments @("up", "-d", "--wait") *> $null
        Write-Output "FAULT_CLEANUP=STACK_RECOVERED"
    }
}
