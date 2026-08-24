[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$normalProject = "compose"
$acceptanceProject = "ledgersync-acceptance-$((Get-Date).ToUniversalTime().ToString('yyyyMMddHHmmss'))-$([Guid]::NewGuid().ToString('N').Substring(0, 8))"
$acceptanceRoot = [IO.Path]::GetFullPath((Join-Path $repositoryRoot "data\local-acceptance"))
$acceptanceState = [IO.Path]::GetFullPath((Join-Path $acceptanceRoot $acceptanceProject))
$normalWasStopped = $false
$acceptanceCreated = $false
$startedAt = [DateTimeOffset]::UtcNow

function Remove-LedgerSyncAccountJourneyState {
    if (-not (Test-Path -LiteralPath $acceptanceState)) { return }
    $expected = [IO.Path]::GetFullPath((Join-Path $acceptanceRoot $acceptanceProject))
    if (-not $acceptanceState.Equals($expected, [StringComparison]::OrdinalIgnoreCase) -or
        (Split-Path -Leaf $acceptanceState) -cnotmatch '^ledgersync-acceptance-\d{14}-[0-9a-f]{8}$') {
        throw "Account-journey cleanup refused an unexpected state path."
    }
    $item = Get-Item -LiteralPath $acceptanceState -Force
    if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Account-journey cleanup refused a reparse-point state path."
    }
    Remove-Item -LiteralPath $acceptanceState -Recurse -Force
}

try {
    $dockerResources = (& docker info --format '{{.ServerVersion}}|{{.NCPU}}|{{.MemTotal}}').Trim().Split('|')
    if ($LASTEXITCODE -ne 0 -or $dockerResources.Count -ne 3) { throw "Docker Desktop is unavailable." }
    if ([int]$dockerResources[1] -lt 2 -or [int64]$dockerResources[2] -lt 4GB) {
        throw "The isolated account journey requires at least two Docker CPUs and 4 GiB of Docker memory."
    }
    $collision = @(& docker ps -a --filter "label=com.docker.compose.project=$acceptanceProject" --format '{{.ID}}')
    $volumeCollision = @(& docker volume ls --filter "label=com.docker.compose.project=$acceptanceProject" --format '{{.Name}}')
    if ($collision.Count -ne 0 -or $volumeCollision.Count -ne 0 -or (Test-Path -LiteralPath $acceptanceState)) {
        throw "Account journey refused a project, volume, or state-path collision."
    }

    . (Join-Path $PSScriptRoot "local-runtime-common.ps1")
    . (Join-Path $PSScriptRoot "local-acceptance-common.ps1")
    if ($script:LedgerSyncComposeProject -cne $normalProject) {
        throw "Account journey must begin from the normal Compose project."
    }
    Assert-LedgerSyncDockerAvailable
    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy
    Invoke-LedgerSyncWebSmoke
    $normalBefore = Get-LedgerSyncFinancialFingerprint

    & pwsh -NoProfile -File (Join-Path $PSScriptRoot "stop-local.ps1") | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "Could not stop the normal project without deleting data." }
    $normalWasStopped = $true

    $env:LEDGERSYNC_LOCAL_COMPOSE_PROJECT = $acceptanceProject
    $env:LEDGERSYNC_LOCAL_STATE_DIRECTORY = $acceptanceState
    . (Join-Path $PSScriptRoot "local-runtime-common.ps1")
    . (Join-Path $PSScriptRoot "local-acceptance-common.ps1")
    New-Item -ItemType Directory -Path $acceptanceRoot -Force | Out-Null
    & pwsh -NoProfile -File (Join-Path $PSScriptRoot "start-local.ps1") | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "Isolated account-journey stack did not start." }
    $acceptanceCreated = $true
    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy
    Invoke-LedgerSyncWebSmoke

    $env:LEDGERSYNC_SYSTEM_ALLOW_LEDGER_MUTATION = "true"
    $env:LEDGERSYNC_SYSTEM_ISOLATED_PROJECT = "true"
    $env:LEDGERSYNC_SYSTEM_WEB_URL = "http://127.0.0.1:3000"
    $env:LEDGERSYNC_SYSTEM_COMPOSE_PROJECT = $acceptanceProject
    $env:LEDGERSYNC_SYSTEM_SEEDED_SOURCE_ACCOUNT_ID = "10000000-0000-4000-8000-000000000001"
    $env:LEDGERSYNC_SYSTEM_RUN_ID = "p3-$([Guid]::NewGuid().ToString('N').Substring(0, 12))"
    Push-Location (Join-Path $repositoryRoot "web")
    try {
        & npm run test:e2e:real-stack
        if ($LASTEXITCODE -ne 0) { throw "Real-stack account product journey failed." }
    }
    finally {
        Pop-Location
    }

    # Docker Compose emits normal lifecycle progress on stderr. Capture it and
    # let Invoke-LedgerSyncCompose enforce the native exit code explicitly.
    $nativeErrorPreference = $PSNativeCommandUseErrorActionPreference
    $commandErrorPreference = $ErrorActionPreference
    $previousComposeProgress = [Environment]::GetEnvironmentVariable("COMPOSE_PROGRESS", "Process")
    $PSNativeCommandUseErrorActionPreference = $false
    $ErrorActionPreference = "Continue"
    $env:COMPOSE_PROGRESS = "quiet"
    try {
        $summary = Invoke-LedgerSyncAcceptanceReconciliation -TenantID "00000000-0000-4000-8000-000000000001"
    }
    finally {
        $PSNativeCommandUseErrorActionPreference = $nativeErrorPreference
        $ErrorActionPreference = $commandErrorPreference
        if ($null -eq $previousComposeProgress) {
            Remove-Item Env:COMPOSE_PROGRESS -ErrorAction SilentlyContinue
        }
        else {
            $env:COMPOSE_PROGRESS = $previousComposeProgress
        }
    }
    $healthyReconciliationStatuses = @("completed", "matched", "passed")
    if ([int64]$summary.outbox_pending -ne 0 -or [int64]$summary.outbox_dead -ne 0 -or
        [int64]$summary.reconciliation_mismatches -ne 0 -or
        $summary.reconciliation_status -notin $healthyReconciliationStatuses) {
        throw "Isolated final operational gate did not pass."
    }

    $elapsed = [Math]::Round(([DateTimeOffset]::UtcNow - $startedAt).TotalSeconds, 2)
    Write-Output "ACCOUNT_PRODUCT_JOURNEY=PASS"
    Write-Output "ISOLATED_PROJECT=$acceptanceProject"
    Write-Output "MIGRATION_VERSION=$($summary.migration_version)"
    Write-Output "OUTBOX=pending:$($summary.outbox_pending),dead:$($summary.outbox_dead)"
    Write-Output "RECONCILIATION=status:$($summary.reconciliation_status),mismatches:$($summary.reconciliation_mismatches)"
    Write-Output "ELAPSED_SECONDS=$elapsed"
}
catch {
    Write-Error $_
    exit 1
}
finally {
    $PSNativeCommandUseErrorActionPreference = $false
    foreach ($name in @(
        "LEDGERSYNC_SYSTEM_ALLOW_LEDGER_MUTATION",
        "LEDGERSYNC_SYSTEM_ISOLATED_PROJECT",
        "LEDGERSYNC_SYSTEM_WEB_URL",
        "LEDGERSYNC_SYSTEM_COMPOSE_PROJECT",
        "LEDGERSYNC_SYSTEM_SEEDED_SOURCE_ACCOUNT_ID",
        "LEDGERSYNC_SYSTEM_RUN_ID"
    )) {
        Remove-Item "Env:$name" -ErrorAction SilentlyContinue
    }
    if ($acceptanceCreated) {
        if ($acceptanceProject -cmatch '^ledgersync-acceptance-\d{14}-[0-9a-f]{8}$') {
            & docker compose --env-file (Join-Path $acceptanceState "runtime.env") `
                -p $acceptanceProject -f (Join-Path $repositoryRoot "deploy\compose\docker-compose.yml") `
                down --volumes --remove-orphans --timeout 10 | Out-Host
            Write-Output "ISOLATED_CLEANUP=PASS"
        }
        else {
            Write-Error "Account-journey cleanup refused an unexpected project name."
        }
    }
    Remove-LedgerSyncAccountJourneyState
    Remove-Item Env:LEDGERSYNC_LOCAL_COMPOSE_PROJECT -ErrorAction SilentlyContinue
    Remove-Item Env:LEDGERSYNC_LOCAL_STATE_DIRECTORY -ErrorAction SilentlyContinue
    if ($normalWasStopped) {
        & pwsh -NoProfile -File (Join-Path $PSScriptRoot "start-local.ps1") -SkipBuild | Out-Host
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Account journey completed but the normal local stack could not be restored."
        }
        else {
            . (Join-Path $PSScriptRoot "local-runtime-common.ps1")
            Assert-LedgerSyncLongRunningServicesHealthy
            Invoke-LedgerSyncWebSmoke
            Compare-LedgerSyncFinancialFingerprint -Before $normalBefore -After (Get-LedgerSyncFinancialFingerprint)
            Write-Output "NORMAL_PROJECT_RESTORED=PASS"
        }
    }
}
