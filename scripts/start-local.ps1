[CmdletBinding()]
param(
    [ValidateRange(30, 600)]
    [int]$WaitTimeoutSeconds = 240,
    [switch]$SkipBuild
)

. (Join-Path $PSScriptRoot "local-runtime-common.ps1")

try {
    Write-Host "Checking Docker and the LedgerSync local boundary..."
    Assert-LedgerSyncDockerAvailable
    Test-LedgerSyncPortAvailableOrOwned
    Invoke-LedgerSyncCompose -ComposeArguments @("config", "-q")

    $upArguments = @("up", "-d")
    if (-not $SkipBuild) {
        $upArguments += "--build"
    }
    $upArguments += @("--wait", "--wait-timeout", [string]$WaitTimeoutSeconds)

    Write-Host "Starting PostgreSQL, Redis, migrations, demo seed, API, worker, and web..."
    Invoke-LedgerSyncCompose -ComposeArguments $upArguments
    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy

    Write-Host "Checking the browser/BFF path and authorized read endpoints..."
    Invoke-LedgerSyncWebSmoke

    Write-Host ""
    Write-Host "LedgerSync local MVP is ready: $script:LedgerSyncWebUrl" -ForegroundColor Green
    Write-Host "Project: $script:LedgerSyncComposeProject | Data volumes were preserved."
}
catch {
    Write-Error $_
    Write-Host "Run scripts/status-local.ps1, then scripts/logs-local.ps1 -Service <name> for bounded diagnostics."
    exit 1
}
