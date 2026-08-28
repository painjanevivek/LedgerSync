[CmdletBinding()]
param(
    [ValidateRange(30, 600)]
    [int]$WaitTimeoutSeconds = 240,
    [switch]$SkipBuild,
    [ValidateSet("demo", "empty")]
    [string]$InitializationMode
)

. (Join-Path $PSScriptRoot "local-runtime-common.ps1")
. (Join-Path $PSScriptRoot "local-backup-common.ps1")
. (Join-Path $PSScriptRoot "local-initialization-common.ps1")

try {
    Write-Host "Checking PowerShell, Git, Docker, Compose, disk space, and the LedgerSync local boundary..."
    Assert-LedgerSyncLocalPrerequisites
    Initialize-LedgerSyncLocalSecrets
    $requestedMode = if ($PSBoundParameters.ContainsKey("InitializationMode")) { $InitializationMode } else { $null }
    $resolvedMode = Initialize-LedgerSyncInitializationMode -RequestedMode $requestedMode
    $env:LEDGERSYNC_INITIALIZATION_MODE = $resolvedMode
    Initialize-LedgerSyncLocalRecoveryEvidenceIndex | Out-Null
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
    Write-Host "Project: $script:LedgerSyncComposeProject | Initialization: $resolvedMode | Data volumes were preserved."
}
catch {
    Write-Error $_
    Write-Host "Run scripts/doctor-local.ps1 for prerequisites, then scripts/status-local.ps1 for service-specific recovery actions."
    exit 1
}
