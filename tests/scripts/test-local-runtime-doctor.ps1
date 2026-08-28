[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
. (Join-Path $repositoryRoot "scripts\local-runtime-common.ps1")

function Assert-LocalRuntimeDoctor {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}

$notInstalled = Resolve-LedgerSyncDockerFailure -Details "docker: command not found"
$stopped = Resolve-LedgerSyncDockerFailure -Details "error during connect: open //./pipe/docker_engine: The system cannot find the file specified"
$denied = Resolve-LedgerSyncDockerFailure -Details "permission denied while trying to connect"
Assert-LocalRuntimeDoctor ($notInstalled -match "engine health check failed") "Unknown Docker errors were not fail-closed."
Assert-LocalRuntimeDoctor ($stopped -match "engine is stopped") "Stopped Docker engine was not distinguished."
Assert-LocalRuntimeDoctor ($denied -match "cannot access the engine") "Docker permission failure was not distinguished."

foreach ($service in @($script:LedgerSyncLongRunningServices + $script:LedgerSyncOneShotServices)) {
    $guidance = Get-LedgerSyncServiceRecoveryGuidance -Service $service -State "failed" -Health "unhealthy" -ExitCode 1
    Assert-LocalRuntimeDoctor (-not [string]::IsNullOrWhiteSpace($guidance.Impact)) "Missing impact for $service."
    Assert-LocalRuntimeDoctor ($guidance.NextAction -match "scripts/") "Missing exact command for $service."
}

$doctorSource = Get-Content -LiteralPath (Join-Path $repositoryRoot "scripts\doctor-local.ps1") -Raw
Assert-LocalRuntimeDoctor ($doctorSource -match "read-only" -and $doctorSource -match "No containers, volumes, data, or secret files were changed") "Doctor does not declare and preserve its read-only boundary."
$composeSource = Get-Content -LiteralPath (Join-Path $repositoryRoot "deploy\compose\docker-compose.yml") -Raw
Assert-LocalRuntimeDoctor ($composeSource -match '127\.0\.0\.1:3000:3000') "Web is not bound to IPv4 loopback."
foreach ($privatePort in @("5432", "6379", "8080")) {
    $publishedPattern = '(?m)^\s*-?\s*["'']?0\.0\.0\.0:{0}' -f $privatePort
    Assert-LocalRuntimeDoctor ($composeSource -notmatch $publishedPattern) "Private port $privatePort was published."
}
Assert-LocalRuntimeDoctor (([regex]::Matches($composeSource, 'stop_grace_period:\s*30s')).Count -eq 5) "Long-running services do not share the graceful shutdown window."

$seedSource = Get-Content -LiteralPath (Join-Path $repositoryRoot "deploy\compose\demo-seed.sql") -Raw
Assert-LocalRuntimeDoctor ($seedSource -match "local_demo_seed_metadata" -and $seedSource -match "persisted_version > 1") "Demo seed version compatibility is not fail-closed."
$resetSource = Get-Content -LiteralPath (Join-Path $repositoryRoot "scripts\reset-local.ps1") -Raw
Assert-LocalRuntimeDoctor ($resetSource -match "Latest validated backup" -and $resetSource -match "No validated LedgerSync backup exists") "Reset does not disclose backup state."
$logsSource = Get-Content -LiteralPath (Join-Path $repositoryRoot "scripts\logs-local.ps1") -Raw
Assert-LocalRuntimeDoctor ($logsSource -match '"--timestamps"') "Bounded logs omit correlation-friendly timestamps."

Write-Output "LOCAL_RUNTIME_DOCTOR_TESTS=PASS"
Write-Output "DOCKER_FAILURE_CLASSIFICATION=PASS"
Write-Output "SERVICE_RECOVERY_GUIDANCE=PASS"
Write-Output "LOOPBACK_AND_GRACEFUL_SHUTDOWN=PASS"
Write-Output "SEED_AND_RESET_SAFETY=PASS"
