[CmdletBinding()]
param(
    [string]$ComposeProject,
    [string]$TenantId = "00000000-0000-4000-8000-000000000001",
    [string]$BackupDirectory,
    [string]$BackupRoot,
    [ValidateRange(1, 100)]
    [int]$RetentionCount = 5,
    [switch]$ValidateOnly,
    [switch]$SkipCorruptionGuard
)

if (-not [string]::IsNullOrWhiteSpace($ComposeProject)) {
    $env:LEDGERSYNC_LOCAL_COMPOSE_PROJECT = $ComposeProject
}

. (Join-Path $PSScriptRoot "local-runtime-common.ps1")
. (Join-Path $PSScriptRoot "local-backup-common.ps1")

$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

$restoreProject = "ledgersync-restore-$((Get-Date).ToUniversalTime().ToString('yyyyMMddHHmmss'))-$([Guid]::NewGuid().ToString('N').Substring(0, 8))"
$restoreComposeFile = Join-Path $script:LedgerSyncRepositoryRoot "deploy\compose\docker-compose.restore.yml"
$restoreCreated = $false
$startedAt = [DateTimeOffset]::UtcNow
$resolvedBackupRoot = $null

function Invoke-LedgerSyncRestoreCompose {
    param(
        [Parameter(Mandatory = $true)][string[]]$Arguments,
        [switch]$CaptureOutput
    )

    $base = @("compose", "-p", $restoreProject, "-f", $restoreComposeFile)
    if ($CaptureOutput) {
        $output = @(& docker @base @Arguments 2>&1)
        if ($LASTEXITCODE -ne 0) {
            throw "Isolated restore command failed: docker $($base -join ' ') $($Arguments -join ' ')`n$($output -join [Environment]::NewLine)"
        }
        return $output
    }
    & docker @base @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Isolated restore command failed: docker $($base -join ' ') $($Arguments -join ' ')"
    }
}

function Get-LedgerSyncRestoreEvidence {
    param([Parameter(Mandatory = $true)][string]$PostgresContainer)

    $sql = @"
SELECT json_build_object(
  'migration_version', COALESCE((SELECT max(version) FROM schema_migrations), 'none'),
  'migration_count', (SELECT count(*) FROM schema_migrations),
  'accounts', (SELECT count(*) FROM accounts),
  'transfers', (SELECT count(*) FROM transfers),
  'ledger_postings', (SELECT count(*) FROM ledger_postings),
  'invalid_journals', (SELECT count(*) FROM (
    SELECT journal_transaction_id
    FROM ledger_postings
    GROUP BY journal_transaction_id, currency
    HAVING count(*) <> 2
       OR sum(CASE WHEN direction='debit' THEN amount_minor ELSE 0 END)
        <> sum(CASE WHEN direction='credit' THEN amount_minor ELSE 0 END)
  ) invalid),
  'invalid_posted_transfers', (SELECT count(*)
    FROM transfers t
    LEFT JOIN journal_transactions j ON j.id=t.journal_transaction_id AND j.transfer_id=t.id
    WHERE t.status='posted' AND j.id IS NULL),
  'negative_balances', (SELECT count(*)
    FROM account_balance_projections balance
    JOIN accounts account ON account.id=balance.account_id
    WHERE (balance.available_minor < 0 OR balance.ledger_minor < 0)
      AND account.account_kind <> 'funding_clearing')
)::text;
"@
    $output = @(& docker exec $PostgresContainer psql -U ledgersync -d ledgersync -Atc $sql 2>&1)
    if ($LASTEXITCODE -ne 0) {
        throw "Could not read bounded evidence from the isolated restore database."
    }
    $json = @($output | Where-Object { ([string]$_).TrimStart().StartsWith("{") } | Select-Object -Last 1)
    if ($json.Count -ne 1) {
        throw "Isolated restore evidence did not return one bounded result."
    }
    return ([string]$json[0] | ConvertFrom-Json)
}

function Get-LedgerSyncComposeStateFingerprint {
    $rows = @(Get-LedgerSyncComposeRows | Sort-Object Service)
    return (($rows | ForEach-Object {
        "$($_.Service)|$($_.ID)|$($_.State)|$($_.Health)|$($_.ExitCode)"
    }) -join "`n")
}

function Get-LedgerSyncRestoreProjectResources {
    $containers = @(& docker ps -a --filter "label=com.docker.compose.project=$restoreProject" --format '{{.ID}}')
    if ($LASTEXITCODE -ne 0) { throw "Could not inspect isolated restore containers." }
    $volumes = @(& docker volume ls --filter "label=com.docker.compose.project=$restoreProject" --format '{{.Name}}')
    if ($LASTEXITCODE -ne 0) { throw "Could not inspect isolated restore volumes." }
    $networks = @(& docker network ls --filter "label=com.docker.compose.project=$restoreProject" --format '{{.ID}}')
    if ($LASTEXITCODE -ne 0) { throw "Could not inspect isolated restore networks." }
    return [pscustomobject]@{
        Containers = @($containers | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) })
        Volumes = @($volumes | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) })
        Networks = @($networks | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) })
    }
}

try {
    Assert-LedgerSyncDockerAvailable
    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy

    $resolvedBackupRoot = Resolve-LedgerSyncBackupRoot -BackupRoot $BackupRoot
    if (-not [string]::IsNullOrWhiteSpace($BackupDirectory) -and
        [string]::IsNullOrWhiteSpace($BackupRoot)) {
        # Explicit bundles remain bound to the canonical default root unless
        # the operator explicitly supplies a different dedicated root.
        $BackupRoot = $resolvedBackupRoot
    }

    if ([string]::IsNullOrWhiteSpace($BackupDirectory)) {
        $backupCommand = @(
            "-NoProfile", "-File", (Join-Path $PSScriptRoot "backup-local.ps1"),
            "-RetentionCount", [string]$RetentionCount
        )
        if (-not [string]::IsNullOrWhiteSpace($BackupRoot)) {
            $backupCommand += @("-BackupRoot", $BackupRoot)
        }
        $backupOutput = @(& pwsh @backupCommand 2>&1)
        if ($LASTEXITCODE -ne 0) {
            throw "Could not create the restore source backup.`n$($backupOutput -join [Environment]::NewLine)"
        }
        $directoryLine = @($backupOutput | Where-Object { [string]$_ -like "BACKUP_DIRECTORY=*" } | Select-Object -Last 1)
        if ($directoryLine.Count -ne 1) {
            throw "Backup command did not return an exact backup directory."
        }
        $BackupDirectory = ([string]$directoryLine[0]).Substring("BACKUP_DIRECTORY=".Length)
    }

    # Integrity validation, including the mutation proof, is completed before an
    # isolated database or volume is created.
    $resolvedBackupRoot = Resolve-LedgerSyncBackupRoot -BackupRoot $BackupRoot
    $backup = Assert-LedgerSyncBackupBundle `
        -BackupDirectory $BackupDirectory -BackupRoot $resolvedBackupRoot
    if (-not $SkipCorruptionGuard) {
        Test-LedgerSyncBackupCorruptionGuard `
            -BackupDirectory $backup.Directory -BackupRoot $resolvedBackupRoot | Out-Null
        Write-Output "CORRUPTION_GUARD=PASS"
    }
    Write-Output "BACKUP_VALIDATION=PASS"
    if ($ValidateOnly) {
        $validationIndex = Write-LedgerSyncRecoveryEvidenceIndex `
            -BackupRoot $resolvedBackupRoot -RetentionCount $RetentionCount
        Write-Output "RECOVERY_EVIDENCE_INDEX=$script:LedgerSyncRecoveryEvidenceFileName"
        Write-Output "RECOVERY_EVIDENCE_JSON=$($validationIndex | ConvertTo-Json -Depth 8 -Compress)"
        Write-Output "VALIDATE_ONLY=PASS"
        return
    }

    $normalBefore = Get-LedgerSyncFinancialFingerprint
    $normalComposeStateBefore = Get-LedgerSyncComposeStateFingerprint
    $normalVolumesBefore = @(docker volume ls `
        --filter "label=com.docker.compose.project=$script:LedgerSyncComposeProject" `
        --format '{{.Name}}' | Sort-Object)
    if ($LASTEXITCODE -ne 0 -or $normalVolumesBefore.Count -lt 1) {
        throw "Could not resolve the normal project's named volumes."
    }

    $apiContainerOutput = @(Invoke-LedgerSyncCompose -ComposeArguments @("ps", "-q", "api") -CaptureOutput)
    $apiContainer = ([string]($apiContainerOutput | Select-Object -Last 1)).Trim()
    if ($apiContainer -cnotmatch '^[0-9a-f]{12,64}$') {
        throw "Could not resolve the exact local API container."
    }
    $apiImage = (& docker inspect --format '{{.Config.Image}}' $apiContainer).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($apiImage)) {
        throw "Could not resolve the migration/reconciliation image."
    }
    $env:LEDGERSYNC_RECOVERY_IMAGE = $apiImage

    $collision = Get-LedgerSyncRestoreProjectResources
    if ($collision.Containers.Count -ne 0 -or $collision.Volumes.Count -ne 0 -or $collision.Networks.Count -ne 0) {
        throw "The uniquely generated isolated restore project collided with existing Docker resources."
    }
    Write-Output "RESTORE_PROJECT=$restoreProject"
    Invoke-LedgerSyncRestoreCompose -Arguments @("config", "-q")
    $restoreCreated = $true
    Invoke-LedgerSyncRestoreCompose -Arguments @("up", "-d", "--wait", "postgres", "redis")

    $postgresOutput = @(Invoke-LedgerSyncRestoreCompose -Arguments @("ps", "-q", "postgres") -CaptureOutput)
    $postgresContainer = ([string]($postgresOutput | Select-Object -Last 1)).Trim()
    $redisOutput = @(Invoke-LedgerSyncRestoreCompose -Arguments @("ps", "-q", "redis") -CaptureOutput)
    $redisContainer = ([string]($redisOutput | Select-Object -Last 1)).Trim()
    if ($postgresContainer -cnotmatch '^[0-9a-f]{12,64}$' -or $redisContainer -cnotmatch '^[0-9a-f]{12,64}$') {
        throw "Could not resolve the isolated recovery containers."
    }

    Invoke-LedgerSyncFileToContainerCommand -SourcePath $backup.DumpPath `
        -ContainerID $postgresContainer -CommandArguments @(
            "pg_restore", "-U", "ledgersync", "-d", "ledgersync",
            "--no-owner", "--no-privileges", "--exit-on-error"
        )

    Invoke-LedgerSyncRestoreCompose -Arguments @("--profile", "tooling", "run", "--rm", "migrate")
    $restored = Get-LedgerSyncRestoreEvidence -PostgresContainer $postgresContainer

    foreach ($comparison in @(
        @("migration_version", [string]$backup.Manifest.schema.migration_version, [string]$restored.migration_version),
        @("migration_count", [string]$backup.Manifest.schema.migration_count, [string]$restored.migration_count),
        @("accounts", [string]$backup.Manifest.counts.accounts, [string]$restored.accounts),
        @("transfers", [string]$backup.Manifest.counts.transfers, [string]$restored.transfers),
        @("ledger_postings", [string]$backup.Manifest.counts.ledger_postings, [string]$restored.ledger_postings)
    )) {
        if ($comparison[1] -cne $comparison[2]) {
            throw "Restore manifest mismatch for $($comparison[0]): expected $($comparison[1]), got $($comparison[2])."
        }
    }
    foreach ($invariant in @("invalid_journals", "invalid_posted_transfers", "negative_balances")) {
        if ([int64]$restored.$invariant -ne 0) {
            throw "Restored financial invariant failed: $invariant=$($restored.$invariant)."
        }
    }

    $reconciliationOutput = @(Invoke-LedgerSyncRestoreCompose -Arguments @(
        "--profile", "tooling", "run", "--rm", "reconcile",
        "--run", "--rebuild-cache", "--tenant-id", $TenantId
    ) -CaptureOutput)
    $reconciliation = @(& docker exec $postgresContainer psql -U ledgersync -d ledgersync -At -F '|' -c `
        "SELECT status,mismatch_count,id FROM reconciliation_runs ORDER BY started_at DESC LIMIT 1;" 2>&1)
    if ($LASTEXITCODE -ne 0 -or $reconciliation.Count -ne 1) {
        throw "Could not read the isolated reconciliation result."
    }
    $reconciliationParts = ([string]$reconciliation[0]).Split('|')
    if ($reconciliationParts.Count -ne 3 -or $reconciliationParts[0] -notin @("completed", "matched", "passed") -or [int64]$reconciliationParts[1] -ne 0) {
        throw "Isolated reconciliation did not finish with zero mismatches: $($reconciliation -join ' ')"
    }
    $redisKeyCount = [int64]((& docker exec $redisContainer redis-cli DBSIZE).Trim())
    if ($LASTEXITCODE -ne 0 -or $redisKeyCount -lt 1) {
        throw "Redis was not rebuilt from restored PostgreSQL evidence."
    }

    $normalAfter = Get-LedgerSyncFinancialFingerprint
    Compare-LedgerSyncFinancialFingerprint -Before $normalBefore -After $normalAfter
    $normalComposeStateAfter = Get-LedgerSyncComposeStateFingerprint
    if ($normalComposeStateBefore -cne $normalComposeStateAfter) {
        throw "The normal Compose service identity or health state changed during the isolated restore."
    }
    $normalVolumesAfter = @(docker volume ls `
        --filter "label=com.docker.compose.project=$script:LedgerSyncComposeProject" `
        --format '{{.Name}}' | Sort-Object)
    if (($normalVolumesBefore -join '|') -cne ($normalVolumesAfter -join '|')) {
        throw "The normal project's named volume set changed during the isolated restore."
    }

    $elapsedSeconds = [Math]::Round(([DateTimeOffset]::UtcNow - $startedAt).TotalSeconds, 2)
    Write-LedgerSyncRestoreEvidence -Backup $backup `
        -ReconciliationStatus ([string]$reconciliationParts[0]) `
        -LocalRTOSeconds $elapsedSeconds | Out-Null
    $recoveryIndex = Write-LedgerSyncRecoveryEvidenceIndex `
        -BackupRoot $resolvedBackupRoot -RetentionCount $RetentionCount
    Write-Output "RESTORE_DRILL=PASS"
    Write-Output "RESTORE_PROJECT=$restoreProject"
    Write-Output "BACKUP_DIRECTORY=$($backup.Directory)"
    Write-Output "MIGRATION_VERSION=$($restored.migration_version)"
    Write-Output "RESTORED_COUNTS=accounts:$($restored.accounts),transfers:$($restored.transfers),postings:$($restored.ledger_postings)"
    Write-Output "INVARIANTS=invalid_journals:0,invalid_posted_transfers:0,negative_balances:0"
    Write-Output "REDIS_DBSIZE=$redisKeyCount"
    Write-Output "RECONCILIATION=status:$($reconciliationParts[0]),mismatches:$($reconciliationParts[1]),run:$($reconciliationParts[2])"
    Write-Output "NORMAL_PROJECT_UNCHANGED=PASS"
    Write-Output "LOCAL_RTO_SECONDS=$elapsedSeconds"
    Write-Output "RECONCILE=$($reconciliationOutput -join ' ')"
    Write-Output "RECOVERY_EVIDENCE_INDEX=$script:LedgerSyncRecoveryEvidenceFileName"
    Write-Output "RECOVERY_EVIDENCE_JSON=$($recoveryIndex | ConvertTo-Json -Depth 8 -Compress)"
}
catch {
    Write-Error $_
    exit 1
}
finally {
    $PSNativeCommandUseErrorActionPreference = $false
    if ($restoreCreated) {
        if ($restoreProject -cnotmatch '^ledgersync-restore-\d{14}-[0-9a-f]{8}$') {
            Write-Error "Cleanup refused an unexpected restore project name: $restoreProject"
        }
        else {
            & docker compose -p $restoreProject -f $restoreComposeFile down `
                --volumes --remove-orphans --timeout 10 | Out-Null
            $remaining = Get-LedgerSyncRestoreProjectResources
            if ($remaining.Containers.Count -ne 0 -or $remaining.Volumes.Count -ne 0 -or $remaining.Networks.Count -ne 0) {
                Write-Error "Isolated restore cleanup left project-owned Docker resources."
            }
            else {
                Write-Output "CLEANUP=COMPLETE"
            }
        }
    }
    Remove-Item Env:LEDGERSYNC_RECOVERY_IMAGE -ErrorAction SilentlyContinue
}
