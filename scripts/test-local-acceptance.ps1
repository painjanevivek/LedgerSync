[CmdletBinding()]
param(
    [switch]$IncludeCapacity,
    [ValidateRange(1, 50)][int]$CapacityTransactionsPerSecond = 25,
    [ValidatePattern('^[1-9][0-9]*(s|m)$')][string]$CapacityDuration = '5m'
)

$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$normalProject = "compose"
$acceptanceProject = "ledgersync-acceptance-$((Get-Date).ToUniversalTime().ToString('yyyyMMddHHmmss'))-$([Guid]::NewGuid().ToString('N').Substring(0, 8))"
$acceptanceRoot = [IO.Path]::GetFullPath((Join-Path $repositoryRoot "data\local-acceptance"))
$acceptanceState = [IO.Path]::GetFullPath((Join-Path $acceptanceRoot $acceptanceProject))
$backupRoot = Join-Path $acceptanceState "backups"
$normalWasStopped = $false
$acceptanceCreated = $false
$startedAt = [DateTimeOffset]::UtcNow

function Remove-LedgerSyncAcceptanceState {
    if (-not (Test-Path -LiteralPath $acceptanceState)) { return }
    $expected = [IO.Path]::GetFullPath((Join-Path $acceptanceRoot $acceptanceProject))
    if (-not $acceptanceState.Equals($expected, [StringComparison]::OrdinalIgnoreCase) -or
        (Split-Path -Leaf $acceptanceState) -cnotmatch '^ledgersync-acceptance-\d{14}-[0-9a-f]{8}$') {
        throw "Acceptance cleanup refused an unexpected state path."
    }
    $item = Get-Item -LiteralPath $acceptanceState -Force
    if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Acceptance cleanup refused a reparse-point state path."
    }
    Remove-Item -LiteralPath $acceptanceState -Recurse -Force
}

try {
    $branch = (& git -C $repositoryRoot branch --show-current).Trim()
    $commit = (& git -C $repositoryRoot rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -cne "main" -or $commit -cnotmatch '^[0-9a-f]{40}$') {
        throw "Acceptance requires the expected main branch and commit."
    }
    $trackedChanges = @(& git -C $repositoryRoot status --porcelain --untracked-files=no)
    if ($LASTEXITCODE -ne 0 -or $trackedChanges.Count -ne 0) {
        throw "Acceptance requires a clean tracked working tree. Commit the Phase 7 harness first."
    }
    $dockerResources = (& docker info --format '{{.ServerVersion}}|{{.NCPU}}|{{.MemTotal}}').Trim().Split('|')
    if ($LASTEXITCODE -ne 0 -or $dockerResources.Count -ne 3) { throw "Docker Desktop is unavailable." }
    if ([int]$dockerResources[1] -lt 2 -or [int64]$dockerResources[2] -lt 4GB) {
        throw "Clean acceptance requires at least two Docker CPUs and 4 GiB of Docker memory."
    }
    $composeVersion = (& docker compose version --short).Trim()
    $goVersion = (& go version).Trim()
    $nodeVersion = (& node --version).Trim()
    $powerShellVersion = [string]$PSVersionTable.PSVersion
    $collision = @(& docker ps -a --filter "label=com.docker.compose.project=$acceptanceProject" --format '{{.ID}}')
    $volumeCollision = @(& docker volume ls --filter "label=com.docker.compose.project=$acceptanceProject" --format '{{.Name}}')
    if ($collision.Count -ne 0 -or $volumeCollision.Count -ne 0 -or (Test-Path -LiteralPath $acceptanceState)) {
        throw "Acceptance refused a project, volume, or state-path collision."
    }

    . (Join-Path $PSScriptRoot "local-runtime-common.ps1")
    . (Join-Path $PSScriptRoot "local-acceptance-common.ps1")
    Assert-LedgerSyncDockerAvailable
    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy
    Invoke-LedgerSyncWebSmoke
    $normalBefore = Get-LedgerSyncFinancialFingerprint
    & pwsh -NoProfile -File (Join-Path $PSScriptRoot "stop-local.ps1") | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Could not stop the normal project without deleting data." }
    $normalWasStopped = $true

    $env:LEDGERSYNC_LOCAL_COMPOSE_PROJECT = $acceptanceProject
    $env:LEDGERSYNC_LOCAL_STATE_DIRECTORY = $acceptanceState
    . (Join-Path $PSScriptRoot "local-runtime-common.ps1")
    . (Join-Path $PSScriptRoot "local-acceptance-common.ps1")
    New-Item -ItemType Directory -Path $acceptanceRoot -Force | Out-Null
    & pwsh -NoProfile -File (Join-Path $PSScriptRoot "start-local.ps1") | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Clean acceptance stack did not start." }
    $acceptanceCreated = $true
    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy
    Invoke-LedgerSyncWebSmoke

    $session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
    $sessionPayload = Invoke-LedgerSyncAcceptanceGET -Session $session -Path "/api/session"
    Assert-LedgerSyncAcceptance ([string]$sessionPayload.environment -ceq "demo") "Clean acceptance did not use the explicit local demo boundary."
    $csrf = [string]$sessionPayload.csrf_token
    Assert-LedgerSyncAcceptance ($csrf.Length -ge 32) "Clean acceptance session omitted its CSRF value."
    $overview = Invoke-WebRequest -UseBasicParsing -WebSession $session -TimeoutSec 15 -Uri $script:LedgerSyncWebUrl
    Assert-LedgerSyncAcceptance ([int]$overview.StatusCode -eq 200 -and $overview.Content -match 'Exact, explainable internal ledger transfers and balances') "Clean acceptance overview did not render the LedgerSync trust promise."

    $firstTransfer = Test-LedgerSyncAcceptanceJourney -Session $session -CSRFToken $csrf -AmountMinor "123" -VerifyReplay
    Invoke-LedgerSyncAcceptanceReconciliation -TenantID ([string]$sessionPayload.tenant_id) | Out-Null

    Invoke-LedgerSyncCompose -ComposeArguments @("restart", "redis", "outbox-worker", "api", "web") | Out-Null
    Invoke-LedgerSyncCompose -ComposeArguments @("up", "-d", "--wait", "redis", "outbox-worker", "api", "web") | Out-Null
    Invoke-LedgerSyncWebSmoke
    $secondTransfer = Test-LedgerSyncAcceptanceJourney -Session $session -CSRFToken $csrf -AmountMinor "234"

    Invoke-LedgerSyncCompose -ComposeArguments @("restart", "postgres") | Out-Null
    Invoke-LedgerSyncCompose -ComposeArguments @("up", "-d", "--wait", "postgres") | Out-Null
    Invoke-LedgerSyncCompose -ComposeArguments @("run", "--rm", "migrate") | Out-Null
    Invoke-LedgerSyncCompose -ComposeArguments @("up", "-d", "--wait", "api", "outbox-worker", "web") | Out-Null
    Invoke-LedgerSyncWebSmoke
    $thirdTransfer = Test-LedgerSyncAcceptanceJourney -Session $session -CSRFToken $csrf -AmountMinor "345"
    $finalSummary = Invoke-LedgerSyncAcceptanceReconciliation -TenantID ([string]$sessionPayload.tenant_id)

    $capacityResult = "NOT_REQUESTED"
    if ($IncludeCapacity) {
        $capacityOutput = Join-Path $repositoryRoot ".tmp\capacity-phase1\phase7-$CapacityTransactionsPerSecond-$CapacityDuration.json"
        $capacityJSON = @(& pwsh -NoProfile -File (Join-Path $PSScriptRoot "run-capacity-qualification.ps1") `
            -ComposeProject $acceptanceProject -WorkloadShape mixed `
            -TransactionsPerSecond $CapacityTransactionsPerSecond -Duration $CapacityDuration `
            -OutputPath $capacityOutput)
        if ($LASTEXITCODE -ne 0) { throw "Isolated Phase 7 capacity qualification failed." }
        $capacityEvidence = Get-Content -LiteralPath $capacityOutput -Raw | ConvertFrom-Json
        if ($capacityEvidence.decision -cne "pass" -or [int]$capacityEvidence.safety.reconciliation_mismatches -ne 0) {
            throw "Isolated Phase 7 capacity evidence did not pass its safety gates."
        }
        $capacityResult = "PASS:tps=$CapacityTransactionsPerSecond,duration=$CapacityDuration,iterations=$($capacityEvidence.k6.iterations),transfer_p95_ms=$($capacityEvidence.k6.transfer_ms.p95),balance_p95_ms=$($capacityEvidence.k6.balance_ms.p95)"
    }

    $backupOutput = @(& pwsh -NoProfile -File (Join-Path $PSScriptRoot "backup-local.ps1") -BackupRoot $backupRoot -RetentionCount 2)
    if ($LASTEXITCODE -ne 0) { throw "Acceptance backup failed." }
    $backupLine = @($backupOutput | Where-Object { [string]$_ -like "BACKUP_DIRECTORY=*" } | Select-Object -Last 1)
    if ($backupLine.Count -ne 1) { throw "Acceptance backup returned no exact directory." }
    $backupDirectory = ([string]$backupLine[0]).Substring("BACKUP_DIRECTORY=".Length)
    $restoreOutput = @(& pwsh -NoProfile -File (Join-Path $PSScriptRoot "local-restore-drill.ps1") `
        -ComposeProject $acceptanceProject -BackupDirectory $backupDirectory -SkipCorruptionGuard)
    if ($LASTEXITCODE -ne 0 -or ($restoreOutput -join " ") -notmatch 'RESTORE_DRILL=PASS' -or
        ($restoreOutput -join " ") -notmatch 'NORMAL_PROJECT_UNCHANGED=PASS') {
        throw "Acceptance isolated restore did not pass."
    }

    $elapsed = [Math]::Round(([DateTimeOffset]::UtcNow - $startedAt).TotalSeconds, 2)
    Write-Output "LOCAL_ACCEPTANCE=PASS"
    Write-Output "SOURCE_COMMIT=$commit"
    Write-Output "TOOLS=docker:$($dockerResources[0]),compose:$composeVersion,go:$goVersion,node:$nodeVersion,pwsh:$powerShellVersion"
    Write-Output "ACCEPTANCE_PROJECT=$acceptanceProject"
    Write-Output "ISOLATED_STATE=data/local-acceptance/$acceptanceProject"
    Write-Output "MIGRATION_VERSION=$($finalSummary.migration_version)"
    Write-Output "TRANSFERS=$firstTransfer,$secondTransfer,$thirdTransfer"
    Write-Output "IDEMPOTENT_REPLAY=PASS"
    Write-Output "DEPENDENCY_RESTARTS=redis,worker,api,web,postgres"
    Write-Output "RECONCILIATION=status:$($finalSummary.reconciliation_status),mismatches:$($finalSummary.reconciliation_mismatches)"
    Write-Output "BACKUP_RESTORE=PASS"
    Write-Output "CAPACITY=$capacityResult"
    Write-Output "ELAPSED_SECONDS=$elapsed"
}
catch {
    Write-Error $_
    exit 1
}
finally {
    $PSNativeCommandUseErrorActionPreference = $false
    if ($acceptanceCreated) {
        if ($acceptanceProject -cmatch '^ledgersync-acceptance-\d{14}-[0-9a-f]{8}$') {
            & docker compose --env-file (Join-Path $acceptanceState "runtime.env") `
                -p $acceptanceProject -f (Join-Path $repositoryRoot "deploy\compose\docker-compose.yml") `
                down --volumes --remove-orphans --timeout 10 | Out-Null
            Write-Output "ACCEPTANCE_CLEANUP=COMPLETE"
        } else {
            Write-Error "Acceptance cleanup refused an unexpected project name."
        }
    }
    Remove-LedgerSyncAcceptanceState
    Remove-Item Env:LEDGERSYNC_LOCAL_COMPOSE_PROJECT -ErrorAction SilentlyContinue
    Remove-Item Env:LEDGERSYNC_LOCAL_STATE_DIRECTORY -ErrorAction SilentlyContinue
    if ($normalWasStopped) {
        & pwsh -NoProfile -File (Join-Path $PSScriptRoot "start-local.ps1") -SkipBuild | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Acceptance completed but the normal local stack could not be restored."
        } else {
            . (Join-Path $PSScriptRoot "local-runtime-common.ps1")
            Assert-LedgerSyncLongRunningServicesHealthy
            Invoke-LedgerSyncWebSmoke
            Compare-LedgerSyncFinancialFingerprint -Before $normalBefore -After (Get-LedgerSyncFinancialFingerprint)
            Write-Output "NORMAL_PROJECT_RESTORED=PASS"
        }
    }
}
