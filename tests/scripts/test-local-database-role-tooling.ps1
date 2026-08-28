[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repositoryRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\.."))
. (Join-Path $repositoryRoot "scripts\local-runtime-common.ps1")

function Assert-DatabaseRoleTooling {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}

$temporaryParent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$testRoot = Join-Path $temporaryParent "ledgersync-database-role-tooling-$([Guid]::NewGuid().ToString('N'))"
$legacyPath = Join-Path $testRoot "runtime.env"
$upgradedPath = Join-Path $testRoot "upgraded.env"
$malformedPath = Join-Path $testRoot "malformed.env"

try {
    New-Item -ItemType Directory -Path $testRoot | Out-Null
    $legacyValues = [ordered]@{
        POSTGRES_PASSWORD = ("11" * 32)
        LEDGERSYNC_SESSION_SECRET = ("22" * 32)
        LEDGERSYNC_CONSISTENCY_SIGNING_KEY = ("33" * 32)
        LEDGERSYNC_BFF_ASSERTION_SECRET = ("44" * 32)
        LEDGERSYNC_WEB_SESSION_SECRET = ("55" * 32)
        LEDGERSYNC_DEVELOPMENT_API_TOKEN = ("66" * 32)
    }
    $legacyLines = @($legacyValues.GetEnumerator() | ForEach-Object { "$($_.Key)=$($_.Value)" })
    [IO.File]::WriteAllLines($legacyPath, $legacyLines, [Text.UTF8Encoding]::new($false))

    Assert-DatabaseRoleTooling (-not (Test-LedgerSyncRuntimeEnvironmentFile -Path $legacyPath)) "Legacy runtime state unexpectedly satisfied the separated-login contract."
    $upgradedLines = @(New-LedgerSyncRuntimeEnvironmentLines -ExistingPath $legacyPath)
    [IO.File]::WriteAllLines($upgradedPath, $upgradedLines, [Text.UTF8Encoding]::new($false))
    Assert-DatabaseRoleTooling (Test-LedgerSyncRuntimeEnvironmentFile -Path $upgradedPath) "Legacy runtime state was not upgraded to the complete credential contract."

    $upgradedValues = @{}
    foreach ($line in $upgradedLines) {
        if ($line -cnotmatch '^([A-Z0-9_]+)=([a-f0-9]{64})$') { throw "Upgrade produced malformed secret state." }
        $upgradedValues[[string]$Matches[1]] = [string]$Matches[2]
    }
    foreach ($entry in $legacyValues.GetEnumerator()) {
        Assert-DatabaseRoleTooling ([string]$upgradedValues[$entry.Key] -ceq [string]$entry.Value) "Runtime upgrade changed existing credential $($entry.Key)."
    }
    $apiPassword = [string]$upgradedValues.LEDGERSYNC_API_DATABASE_PASSWORD
    $workerPassword = [string]$upgradedValues.LEDGERSYNC_WORKER_DATABASE_PASSWORD
    Assert-DatabaseRoleTooling ($apiPassword -cmatch '^[a-f0-9]{64}$' -and $workerPassword -cmatch '^[a-f0-9]{64}$') "Runtime upgrade did not generate both 32-byte database credentials."
    Assert-DatabaseRoleTooling ($apiPassword -cne $workerPassword -and $apiPassword -cne [string]$upgradedValues.POSTGRES_PASSWORD -and $workerPassword -cne [string]$upgradedValues.POSTGRES_PASSWORD) "Runtime upgrade reused a database credential across trust boundaries."

    $rerunLines = @(New-LedgerSyncRuntimeEnvironmentLines -ExistingPath $upgradedPath)
    Assert-DatabaseRoleTooling (($rerunLines -join "`n") -ceq ($upgradedLines -join "`n")) "A completed runtime upgrade was not idempotent."

    [IO.File]::WriteAllText($malformedPath, "POSTGRES_PASSWORD=not-a-secret", [Text.UTF8Encoding]::new($false))
    $malformedBefore = [IO.File]::ReadAllText($malformedPath)
    $malformedRejected = $false
    try { New-LedgerSyncRuntimeEnvironmentLines -ExistingPath $malformedPath | Out-Null }
    catch { $malformedRejected = $true }
    Assert-DatabaseRoleTooling $malformedRejected "Malformed runtime state was silently replaced."
    Assert-DatabaseRoleTooling ([IO.File]::ReadAllText($malformedPath) -ceq $malformedBefore) "Rejected malformed runtime state was changed."

    $compose = Get-Content -Raw -LiteralPath (Join-Path $repositoryRoot "deploy\compose\docker-compose.yml")
    $ownerDSNMarker = [regex]::Escape('LEDGERSYNC_DATABASE_URL: postgres://ledgersync:${POSTGRES_PASSWORD:')
    Assert-DatabaseRoleTooling ([regex]::Matches($compose, $ownerDSNMarker).Count -eq 1) "Database-owner DSN is not confined to the migration/role-provisioning job."
    Assert-DatabaseRoleTooling ($compose -match 'postgres://ledgersync_local_api:\$\{LEDGERSYNC_API_DATABASE_PASSWORD' -and $compose -match 'postgres://ledgersync_local_worker:\$\{LEDGERSYNC_WORKER_DATABASE_PASSWORD') "Compose does not use distinct workload logins."
    Assert-DatabaseRoleTooling ($compose.IndexOf('/usr/local/bin/migrate &&', [StringComparison]::Ordinal) -lt $compose.IndexOf('-f /database-roles/roles.sql', [StringComparison]::Ordinal)) "Role grants do not follow migrations."

    Write-Output "LOCAL_DATABASE_ROLE_TOOLING_TESTS=PASS"
    Write-Output "LEGACY_RUNTIME_UPGRADE=IDEMPOTENT"
    Write-Output "WORKLOAD_DATABASE_SECRETS=INDEPENDENT"
    Write-Output "OWNER_DSN_LONG_RUNNING_WORKLOADS=ABSENT"
}
finally {
    if (Test-Path -LiteralPath $testRoot -PathType Container) {
        $resolvedCleanup = [IO.Path]::GetFullPath($testRoot)
        if (-not $resolvedCleanup.StartsWith($temporaryParent, [StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetFileName($resolvedCleanup) -cnotmatch '^ledgersync-database-role-tooling-[0-9a-f]{32}$') {
            throw "Refusing to remove an unverified database-role tooling test directory."
        }
        Remove-Item -LiteralPath $resolvedCleanup -Recurse -Force
    }
}
