[CmdletBinding()]
param([switch]$SkipBuild, [switch]$IncludePeer, [switch]$WithScheduler, [ValidateRange(1,2)][int]$APIReplicas = 1, [ValidateRange(1,2)][int]$WorkerReplicas = 1, [ValidateRange(30,600)][int]$WaitTimeoutSeconds = 300)
$ErrorActionPreference = 'Stop'
$studyRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$studyActivePath = Join-Path $studyRoot 'data/unit-study/active.json'
if ($env:LEDGERSYNC_STUDY_ACTIVE_PATH) {
    $studyActivePath = [IO.Path]::GetFullPath($env:LEDGERSYNC_STUDY_ACTIVE_PATH)
    $allowedRoot = [IO.Path]::GetFullPath((Join-Path $studyRoot 'data/unit-study')).TrimEnd('\') + '\'
    if (-not $studyActivePath.StartsWith($allowedRoot,[StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetExtension($studyActivePath) -ne '.json') { throw 'Study state must remain in data/unit-study.' }
}
if (-not (Test-Path -LiteralPath $studyActivePath)) {
    New-Item -ItemType Directory -Force -Path (Split-Path $studyActivePath) | Out-Null
    $project = "ledgersync-acceptance-$([DateTime]::UtcNow.ToString('yyyyMMddHHmmss'))-$([Guid]::NewGuid().ToString('N').Substring(0,8))"
    @{ project = $project; created_at = [DateTime]::UtcNow.ToString('o') } | ConvertTo-Json | Set-Content -LiteralPath $studyActivePath
}
. (Join-Path $PSScriptRoot 'unit-study-common.ps1')
Assert-LedgerSyncDockerAvailable
Initialize-LedgerSyncLocalSecrets
$indexPath = Join-Path $script:LedgerSyncRuntimeStateDirectory 'recovery-evidence-index.json'
if (-not (Test-Path -LiteralPath $indexPath)) {
    @{ format_version = 'ledgersync-recovery-evidence-index/v1'; generated_at_utc = [DateTime]::UtcNow.ToString('o'); latest_backup = $null; latest_restore = $null;
       retention = @{ valid_backup_count = 0; ignored_entry_count = 0; configured_keep_count = 5 } } | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $indexPath
}
Invoke-LedgerSyncCompose -ComposeArguments @('config','-q')
if (-not $SkipBuild) {
    # A migration can change claim contracts. Stop all old study applications
    # before the one-shot migration. The normal workspace is not a target.
    Invoke-LedgerSyncCompose -ComposeArguments @('--profile','scheduler','stop','reconciliation-scheduler','web','web-peer','study-proxy','api','outbox-worker')
}
$arguments = @('up','-d','--wait','--wait-timeout',[string]$WaitTimeoutSeconds)
$arguments += @('--scale',"api=$APIReplicas",'--scale',"outbox-worker=$WorkerReplicas")
if (-not $SkipBuild) { $arguments += '--build' }
if ($IncludePeer) { $arguments += @('postgres','redis','api','outbox-worker','web','web-peer') }
Invoke-LedgerSyncCompose -ComposeArguments $arguments
Assert-LedgerSyncOneShotServicesCompleted
Assert-LedgerSyncLongRunningServicesHealthy
Invoke-LedgerSyncWebSmoke
if ($WithScheduler) {
    Invoke-LedgerSyncAcceptanceSQLJSON -SQL "WITH allowed AS (INSERT INTO reconciliation_scheduler_tenants(workload_name,tenant_id) VALUES('ledgersync_local_scheduler','00000000-0000-4000-8000-000000000001') ON CONFLICT DO NOTHING RETURNING tenant_id) SELECT jsonb_build_object('configured',true);" | Out-Null
    Invoke-LedgerSyncCompose -ComposeArguments @('--profile','scheduler','up','-d','--no-deps','--scale','reconciliation-scheduler=2','reconciliation-scheduler')
}
Write-Host "Study ready at $script:LedgerSyncWebUrl. Project: $script:LedgerSyncComposeProject. Normal data was preserved."
