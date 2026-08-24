[CmdletBinding()]
param(
    [string]$BackupRoot,
    [ValidateRange(1, 100)]
    [int]$RetentionCount = 5
)

. (Join-Path $PSScriptRoot "local-runtime-common.ps1")
. (Join-Path $PSScriptRoot "local-backup-common.ps1")

$partialDirectory = $null
$postgresContainer = $null
$snapshotPrefix = $null

try {
    Assert-LedgerSyncDockerAvailable
    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy

    $resolvedBackupRoot = Resolve-LedgerSyncBackupRoot -BackupRoot $BackupRoot
    New-Item -ItemType Directory -Path $resolvedBackupRoot -Force | Out-Null

    $postgresContainerOutput = @(Invoke-LedgerSyncCompose -ComposeArguments @("ps", "-q", "postgres") -CaptureOutput)
    $postgresContainer = ([string]($postgresContainerOutput | Select-Object -Last 1)).Trim()
    if ($postgresContainer -cnotmatch '^[0-9a-f]{12,64}$') {
        throw "Could not resolve the exact local PostgreSQL container."
    }

    $createdAt = [DateTimeOffset]::UtcNow
    $sourceCommit = (& git -C $script:LedgerSyncRepositoryRoot rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $sourceCommit -cnotmatch '^[0-9a-f]{40}$') {
        throw "Could not bind the backup to a Git commit."
    }

    $backupName = "backup-$($createdAt.ToString('yyyyMMddTHHmmssZ'))-$($sourceCommit.Substring(0, 7))"
    $finalDirectory = Join-Path $resolvedBackupRoot $backupName
    if (Test-Path -LiteralPath $finalDirectory) {
        throw "A finalized backup already exists for this timestamp and commit: $finalDirectory"
    }

    $partialName = ".partial-$([Guid]::NewGuid().ToString('N'))"
    $partialDirectory = Join-Path $resolvedBackupRoot $partialName
    New-Item -ItemType Directory -Path $partialDirectory | Out-Null
    $dumpPath = Join-Path $partialDirectory "database.dump"
    $snapshotPrefix = "/tmp/ledgersync-backup-$([Guid]::NewGuid().ToString('N'))"
    $remoteDumpPath = "$snapshotPrefix.dump"
    $remoteControllerPath = "$snapshotPrefix.sql"
    $remoteSnapshotPath = "$snapshotPrefix.snapshot"
    $remoteEvidencePath = "$snapshotPrefix.evidence"
    $remoteReadyPath = "$snapshotPrefix.ready"
    $remoteReleasePath = "$snapshotPrefix.release"
    $remoteDonePath = "$snapshotPrefix.done"
    $remoteErrorPath = "$snapshotPrefix.error"
    $localControllerPath = Join-Path $partialDirectory "snapshot-controller.sql"
    $evidenceSql = @"
SELECT json_build_object(
  'migration_version', COALESCE((SELECT max(version) FROM schema_migrations), 'none'),
  'migration_count', (SELECT count(*) FROM schema_migrations),
  'accounts', (SELECT count(*) FROM accounts),
  'transfers', (SELECT count(*) FROM transfers),
  'ledger_postings', (SELECT count(*) FROM ledger_postings)
)::text;
"@
    $controllerSql = @"
\set ON_ERROR_STOP on
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SELECT pg_export_snapshot() AS snapshot_id \gset
\o $remoteSnapshotPath
\qecho :snapshot_id
\o
\! touch $remoteReadyPath
\! while [ ! -f $remoteReleasePath ]; do sleep 0.1; done
\o $remoteEvidencePath
$evidenceSql
\o
COMMIT;
\! touch $remoteDonePath
"@
    $controllerSql.Replace("`r", "") | Set-Content -LiteralPath $localControllerPath -Encoding utf8 -NoNewline
    & docker cp $localControllerPath "${postgresContainer}:${remoteControllerPath}" | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Could not stage the consistent-snapshot controller."
    }
    Remove-Item -LiteralPath $localControllerPath -Force

    $controllerCommand = "psql -XAt -v ON_ERROR_STOP=1 -U ledgersync -d ledgersync -f $remoteControllerPath >/dev/null 2>$remoteErrorPath"
    & docker exec -d $postgresContainer sh -c $controllerCommand
    if ($LASTEXITCODE -ne 0) {
        throw "Could not start the consistent PostgreSQL snapshot controller."
    }

    $snapshotReady = $false
    for ($attempt = 0; $attempt -lt 300; $attempt++) {
        $PSNativeCommandUseErrorActionPreference = $false
        & docker exec $postgresContainer test -f $remoteReadyPath *> $null
        $readyExitCode = $LASTEXITCODE
        $PSNativeCommandUseErrorActionPreference = $true
        if ($readyExitCode -eq 0) {
            $snapshotReady = $true
            break
        }
        Start-Sleep -Milliseconds 100
    }
    if (-not $snapshotReady) {
        $controllerError = @(& docker exec $postgresContainer sh -c "cat $remoteErrorPath 2>/dev/null || true")
        throw "PostgreSQL did not export a consistent backup snapshot within 30 seconds. $($controllerError -join ' ')"
    }
    $snapshotOutput = @(& docker exec $postgresContainer sh -c "tr -d '\r\n ' <$remoteSnapshotPath")
    $snapshotId = if ($snapshotOutput.Count -eq 1) { ([string]$snapshotOutput[0]).Trim() } else { "" }
    if ($LASTEXITCODE -ne 0 -or $snapshotId -cnotmatch '^[0-9A-Fa-f-]{8,128}$') {
        throw "PostgreSQL returned a malformed exported snapshot identifier."
    }

    & docker exec $postgresContainer pg_dump `
        -U ledgersync -d ledgersync -Fc --no-owner --no-privileges `
        --snapshot=$snapshotId -f $remoteDumpPath
    if ($LASTEXITCODE -ne 0) {
        throw "PostgreSQL logical backup failed."
    }
    & docker exec $postgresContainer touch $remoteReleasePath
    if ($LASTEXITCODE -ne 0) {
        throw "Could not release the consistent backup snapshot."
    }
    $snapshotDone = $false
    for ($attempt = 0; $attempt -lt 300; $attempt++) {
        $PSNativeCommandUseErrorActionPreference = $false
        & docker exec $postgresContainer test -f $remoteDonePath *> $null
        $doneExitCode = $LASTEXITCODE
        $PSNativeCommandUseErrorActionPreference = $true
        if ($doneExitCode -eq 0) {
            $snapshotDone = $true
            break
        }
        Start-Sleep -Milliseconds 100
    }
    if (-not $snapshotDone) {
        $controllerError = @(& docker exec $postgresContainer sh -c "cat $remoteErrorPath 2>/dev/null || true")
        throw "PostgreSQL snapshot evidence did not finish within 30 seconds. $($controllerError -join ' ')"
    }

    & docker cp "${postgresContainer}:${remoteDumpPath}" $dumpPath | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Could not copy the completed PostgreSQL backup into local protected storage."
    }
    $evidenceJson = @(& docker exec $postgresContainer sh -c "cat $remoteEvidencePath")
    if ($LASTEXITCODE -ne 0 -or $evidenceJson.Count -ne 1 -or -not ([string]$evidenceJson[0]).TrimStart().StartsWith("{")) {
        throw "Backup snapshot evidence did not return one bounded result."
    }
    $evidence = [string]$evidenceJson[0] | ConvertFrom-Json

    $dump = Get-Item -LiteralPath $dumpPath
    $digest = (Get-FileHash -LiteralPath $dumpPath -Algorithm SHA256).Hash.ToLowerInvariant()
    $manifest = [ordered]@{
        format_version = $script:LedgerSyncBackupFormatVersion
        created_at_utc = $createdAt.ToString("o")
        source_commit = $sourceCommit
        scope = [ordered]@{
            deployment = "local-only"
            database = "all local LedgerSync tenants"
            manifest_data = "redacted counts and integrity metadata only"
            currency = "INR"
        }
        schema = [ordered]@{
            migration_version = [string]$evidence.migration_version
            migration_count = [int64]$evidence.migration_count
        }
        database = [ordered]@{
            file_name = "database.dump"
            byte_length = [int64]$dump.Length
            sha256 = $digest
        }
        counts = [ordered]@{
            accounts = [int64]$evidence.accounts
            transfers = [int64]$evidence.transfers
            ledger_postings = [int64]$evidence.ledger_postings
        }
    }
    $manifest | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $partialDirectory "manifest.json") -Encoding utf8
    Assert-LedgerSyncBackupBundle -BackupDirectory $partialDirectory | Out-Null

    Move-Item -LiteralPath $partialDirectory -Destination $finalDirectory
    $partialDirectory = $null

    $finalized = Assert-LedgerSyncBackupBundle -BackupDirectory $finalDirectory
    $retainedBackups = @(Get-ChildItem -LiteralPath $resolvedBackupRoot -Directory |
        Where-Object { $_.Name -cmatch $script:LedgerSyncBackupDirectoryPattern } |
        Sort-Object Name -Descending)
    foreach ($expired in @($retainedBackups | Select-Object -Skip $RetentionCount)) {
        Remove-LedgerSyncValidatedDirectory `
            -Parent $resolvedBackupRoot `
            -Directory $expired.FullName `
            -AllowedLeafPattern $script:LedgerSyncBackupDirectoryPattern
    }

    Write-Output "BACKUP=PASS"
    Write-Output "BACKUP_DIRECTORY=$($finalized.Directory)"
    Write-Output "FORMAT_VERSION=$($finalized.Manifest.format_version)"
    Write-Output "MIGRATION_VERSION=$($finalized.Manifest.schema.migration_version)"
    Write-Output "COUNTS=accounts:$($finalized.Manifest.counts.accounts),transfers:$($finalized.Manifest.counts.transfers),postings:$($finalized.Manifest.counts.ledger_postings)"
    Write-Output "DUMP_BYTES=$($finalized.Manifest.database.byte_length)"
    Write-Output "SHA256=$($finalized.Manifest.database.sha256)"
    Write-Output "RETAINED=$([Math]::Min($retainedBackups.Count, $RetentionCount))"
}
catch {
    Write-Error $_
    exit 1
}
finally {
    $PSNativeCommandUseErrorActionPreference = $false
    if ($postgresContainer -and $snapshotPrefix) {
        & docker exec $postgresContainer touch "$snapshotPrefix.release" *> $null
        for ($cleanupAttempt = 0; $cleanupAttempt -lt 50; $cleanupAttempt++) {
            & docker exec $postgresContainer test -f "$snapshotPrefix.done" *> $null
            if ($LASTEXITCODE -eq 0) {
                break
            }
            Start-Sleep -Milliseconds 100
        }
        & docker exec $postgresContainer sh -c "rm -f $snapshotPrefix.*" *> $null
    }
    if ($partialDirectory -and (Test-Path -LiteralPath $partialDirectory)) {
        $resolvedBackupRoot = Resolve-LedgerSyncBackupRoot -BackupRoot $BackupRoot
        Remove-LedgerSyncValidatedDirectory `
            -Parent $resolvedBackupRoot `
            -Directory $partialDirectory `
            -AllowedLeafPattern '^\.partial-[0-9a-f]{32}$'
    }
}
