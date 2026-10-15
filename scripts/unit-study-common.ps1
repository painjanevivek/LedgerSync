Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$studyRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$studyActivePath = Join-Path $studyRoot 'data/unit-study/active.json'
if ($env:LEDGERSYNC_STUDY_ACTIVE_PATH) {
    $studyActivePath = [IO.Path]::GetFullPath($env:LEDGERSYNC_STUDY_ACTIVE_PATH)
    $allowedRoot = [IO.Path]::GetFullPath((Join-Path $studyRoot 'data/unit-study')).TrimEnd('\') + '\'
    if (-not $studyActivePath.StartsWith($allowedRoot,[StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetExtension($studyActivePath) -ne '.json') { throw 'Study state must remain in data/unit-study.' }
}
if (-not (Test-Path -LiteralPath $studyActivePath -PathType Leaf)) {
    throw 'Run scripts/start-unit-study.ps1 first.'
}
$studyState = Get-Content -LiteralPath $studyActivePath -Raw | ConvertFrom-Json
if ($studyState.project -cnotmatch '^ledgersync-acceptance-\d{14}-[0-9a-f]{8}$') {
    throw 'The study project identity is not valid.'
}
$env:LEDGERSYNC_LOCAL_COMPOSE_PROJECT = $studyState.project
$env:LEDGERSYNC_LOCAL_STATE_DIRECTORY = Join-Path $studyRoot "data/local-acceptance/$($studyState.project)"
. (Join-Path $PSScriptRoot 'local-runtime-common.ps1')
$script:LedgerSyncRuntimeSecretNames += 'LEDGERSYNC_SCHEDULER_DATABASE_PASSWORD'
. (Join-Path $PSScriptRoot 'local-acceptance-common.ps1')
Assert-LedgerSyncAcceptanceProjectIdentity -Project $script:LedgerSyncComposeProject `
    -StateRoot (Join-Path $studyRoot 'data/local-acceptance') -StatePath $script:LedgerSyncRuntimeStateDirectory
$env:LEDGERSYNC_STUDY_STATE = $script:LedgerSyncRuntimeStateDirectory.Replace('\','/')
$env:LEDGERSYNC_STUDY_IMAGE_PREFIX = $script:LedgerSyncComposeProject
$script:LedgerSyncWebUrl = 'http://127.0.0.1:3102'
$script:LedgerSyncStudyComposeFiles = @($script:LedgerSyncComposeFile, (Join-Path $studyRoot 'deploy/compose/docker-compose.study.yml'))

# Reuse the acceptance helpers with a separate Compose overlay and identity.
# The product acceptance script retains its clean-tree and main-branch gates.
function Invoke-LedgerSyncCompose {
    param([Parameter(Mandatory = $true)][string[]]$ComposeArguments, [switch]$CaptureOutput)
    $baseArguments = @('compose','--env-file',$script:LedgerSyncRuntimeEnvironmentFile,'-p',$script:LedgerSyncComposeProject)
    foreach ($file in $script:LedgerSyncStudyComposeFiles) { $baseArguments += @('-f',$file) }
    if ($CaptureOutput) {
        $result = & docker @baseArguments @ComposeArguments 2>&1
        if ($LASTEXITCODE -ne 0) { throw "Study Compose failed: $($ComposeArguments -join ' ')`n$($result -join [Environment]::NewLine)" }
        return $result
    }
    & docker @baseArguments @ComposeArguments
    if ($LASTEXITCODE -ne 0) { throw "Study Compose failed: $($ComposeArguments -join ' ')" }
}

function Export-LedgerSyncStudyEnvironment {
    $env:LEDGERSYNC_SYSTEM_ALLOW_LEDGER_MUTATION = 'true'
    $env:LEDGERSYNC_SYSTEM_ISOLATED_PROJECT = 'true'
    $env:LEDGERSYNC_SYSTEM_WEB_URL = $script:LedgerSyncWebUrl
    $env:LEDGERSYNC_SYSTEM_COMPOSE_PROJECT = $script:LedgerSyncComposeProject
    $env:LEDGERSYNC_SYSTEM_RUN_ID = "study-$([Guid]::NewGuid().ToString('N').Substring(0,12))"
    $env:LEDGERSYNC_SYSTEM_SOURCE_ACCOUNT_ID = '10000000-0000-4000-8000-000000000001'
    $env:LEDGERSYNC_SYSTEM_SEEDED_SOURCE_ACCOUNT_ID = '10000000-0000-4000-8000-000000000001'
    $env:LEDGERSYNC_SYSTEM_PEER_WEB_URL = 'http://127.0.0.1:3103'
}
