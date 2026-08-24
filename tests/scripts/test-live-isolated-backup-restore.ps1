[CmdletBinding()]
param(
    [switch]$ConfirmLiveIsolatedRecovery,
    [ValidateRange(1, 10)][int]$RetentionCount = 2
)

$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true
if (-not $ConfirmLiveIsolatedRecovery) {
    throw "This live test creates and removes an isolated recovery directory/project. Re-run with -ConfirmLiveIsolatedRecovery."
}

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$scriptsRoot = Join-Path $repositoryRoot "scripts"
$restoreComposeFile = Join-Path $repositoryRoot "deploy\compose\docker-compose.restore.yml"
$acceptanceRoot = [IO.Path]::GetFullPath((Join-Path $repositoryRoot "data\local-acceptance"))
$acceptanceName = "ledgersync-recovery-acceptance-$((Get-Date).ToUniversalTime().ToString('yyyyMMddHHmmss'))-$([Guid]::NewGuid().ToString('N').Substring(0, 8))"
$acceptanceState = [IO.Path]::GetFullPath((Join-Path $acceptanceRoot $acceptanceName))
$backupRoot = Join-Path $acceptanceState "backups"
$restoreProject = $null
$failurePhase = "preflight"
$failure = $null
$cleanupFailure = $null
$capturedOutput = [Collections.Generic.List[object]]::new()
$secretValues = [Collections.Generic.List[string]]::new()

. (Join-Path $scriptsRoot "local-runtime-common.ps1")
. (Join-Path $scriptsRoot "local-backup-common.ps1")
. (Join-Path $scriptsRoot "local-api-credential-common.ps1")

function Assert-RecoveryAcceptanceIdentity {
    if ($acceptanceName -cnotmatch '^ledgersync-recovery-acceptance-\d{14}-[0-9a-f]{8}$') {
        throw "Generated recovery acceptance identity is invalid."
    }
    $expected = [IO.Path]::GetFullPath((Join-Path $acceptanceRoot $acceptanceName))
    if (-not $acceptanceState.Equals($expected, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Generated recovery acceptance state path is invalid."
    }
}

function Invoke-CapturedScript {
    param([Parameter(Mandatory = $true)][string]$Path, [string[]]$Arguments)
    $nativePreference = $PSNativeCommandUseErrorActionPreference
    try {
        $PSNativeCommandUseErrorActionPreference = $false
        $output = @(& pwsh -NoProfile -File $Path @Arguments *>&1)
        $exitCode = $LASTEXITCODE
    }
    finally {
        $PSNativeCommandUseErrorActionPreference = $nativePreference
    }
    foreach ($item in $output) { $capturedOutput.Add($item) }
    return [pscustomobject]@{ Output = $output; ExitCode = $exitCode }
}

function Get-ComposeStateFingerprint {
    return ((@(Get-LedgerSyncComposeRows | Sort-Object Service) | ForEach-Object {
        "$($_.Service)|$($_.ID)|$($_.State)|$($_.Health)|$($_.ExitCode)"
    }) -join "`n")
}

function Get-RestoreProjectResources {
    param([Parameter(Mandatory = $true)][string]$Project)
    $containers = @(& docker ps -a --filter "label=com.docker.compose.project=$Project" --format '{{.ID}}')
    $volumes = @(& docker volume ls --filter "label=com.docker.compose.project=$Project" --format '{{.Name}}')
    $networks = @(& docker network ls --filter "label=com.docker.compose.project=$Project" --format '{{.ID}}')
    return [pscustomobject]@{
        Containers = @($containers | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) })
        Volumes = @($volumes | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) })
        Networks = @($networks | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) })
    }
}

function Assert-CapturedOutputSanitized {
    $text = ($capturedOutput | ForEach-Object { [string]$_ }) -join "`n"
    foreach ($secret in $secretValues) {
        if (-not [string]::IsNullOrWhiteSpace($secret) -and
            $text.Contains($secret, [StringComparison]::Ordinal)) {
            throw "Live recovery output disclosed protected runtime credential material."
        }
    }
    $jsonLines = @($capturedOutput | ForEach-Object { [string]$_ } | Where-Object { $_ -like "RECOVERY_EVIDENCE_JSON=*" })
    foreach ($line in $jsonLines) {
        $json = $line.Substring("RECOVERY_EVIDENCE_JSON=".Length)
        if ($json -match '(?i)([a-z]:\\|/home/|/users/|database\.dump|password|secret|token|sha256\s*":\s*"[a-f0-9]{64})') {
            throw "Live recovery summary contained prohibited path or credential material."
        }
    }
}

function Remove-RecoveryAcceptanceState {
    if (-not (Test-Path -LiteralPath $acceptanceState)) { return }
    Assert-RecoveryAcceptanceIdentity
    $item = Get-Item -LiteralPath $acceptanceState -Force
    if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Recovery acceptance cleanup refused a reparse-point state path."
    }
    Remove-Item -LiteralPath $acceptanceState -Recurse -Force
}

try {
    Assert-RecoveryAcceptanceIdentity
    if (-not [string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable("LEDGERSYNC_LOCAL_COMPOSE_PROJECT")) -or
        -not [string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable("LEDGERSYNC_LOCAL_STATE_DIRECTORY"))) {
        throw "Live recovery acceptance requires unset project/state overrides."
    }
    Assert-LedgerSyncDockerAvailable
    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy
    Invoke-LedgerSyncWebSmoke
    if (Test-Path -LiteralPath $acceptanceState) {
        throw "Generated recovery acceptance state already exists."
    }
    $normalBefore = Get-LedgerSyncFinancialFingerprint
    $normalStateBefore = Get-ComposeStateFingerprint
    $normalVolumesBefore = @(& docker volume ls `
        --filter "label=com.docker.compose.project=$script:LedgerSyncComposeProject" --format '{{.Name}}' | Sort-Object)

    $runtime = Read-LedgerSyncProtectedRuntimeEnvironment -Path $script:LedgerSyncRuntimeEnvironmentFile
    foreach ($value in $runtime.Values.Values) { $secretValues.Add([string]$value) }

    $failurePhase = "streamed protected backup"
    New-Item -ItemType Directory -Path $acceptanceState -Force | Out-Null
    $backupResult = Invoke-CapturedScript -Path (Join-Path $scriptsRoot "backup-local.ps1") -Arguments @(
        "-BackupRoot", $backupRoot, "-RetentionCount", [string]$RetentionCount
    )
    if ($backupResult.ExitCode -ne 0) { throw "Live isolated backup command failed." }
    $backupDirectoryLine = @($backupResult.Output | Where-Object { [string]$_ -like "BACKUP_DIRECTORY=*" } | Select-Object -Last 1)
    if ($backupDirectoryLine.Count -ne 1) { throw "Live backup omitted its exact bundle location." }
    $backupDirectory = ([string]$backupDirectoryLine[0]).Substring("BACKUP_DIRECTORY=".Length)
    $backup = Assert-LedgerSyncBackupBundle -BackupDirectory $backupDirectory -BackupRoot $backupRoot
    if ($env:OS -eq "Windows_NT") {
        foreach ($path in @($backup.DumpPath, $backup.ManifestPath, (Join-Path $backupRoot $script:LedgerSyncRecoveryEvidenceFileName))) {
            if (-not (Get-Acl -LiteralPath $path).AreAccessRulesProtected) {
                throw "Live backup or recovery index file is not protected."
            }
        }
    }

    $failurePhase = "isolated restore drill"
    $restoreResult = Invoke-CapturedScript -Path (Join-Path $scriptsRoot "local-restore-drill.ps1") -Arguments @(
        "-ComposeProject", "compose", "-BackupDirectory", $backupDirectory,
        "-BackupRoot", $backupRoot, "-RetentionCount", [string]$RetentionCount
    )
    $restoreProjectLine = @($restoreResult.Output | Where-Object { [string]$_ -like "RESTORE_PROJECT=*" } | Select-Object -First 1)
    if ($restoreProjectLine.Count -eq 1) {
        $restoreProject = ([string]$restoreProjectLine[0]).Substring("RESTORE_PROJECT=".Length)
    }
    if ($restoreResult.ExitCode -ne 0 -or
        ($restoreResult.Output -join "`n") -notmatch '(?m)^RESTORE_DRILL=PASS$' -or
        ($restoreResult.Output -join "`n") -notmatch '(?m)^NORMAL_PROJECT_UNCHANGED=PASS$' -or
        ($restoreResult.Output -join "`n") -notmatch '(?m)^CLEANUP=COMPLETE$') {
        throw "Live isolated restore did not return all required evidence."
    }
    if ($restoreProject -cnotmatch '^ledgersync-restore-\d{14}-[0-9a-f]{8}$') {
        throw "Live restore did not use a uniquely named bounded project."
    }

    $index = Read-LedgerSyncRecoveryEvidenceIndex -BackupRoot $backupRoot
    if ($index.latest_backup.validation_status -cne "passed" -or
        $index.latest_backup.digest_status -cne "verified" -or
        $index.latest_restore.status -cne "passed" -or
        [int]$index.latest_restore.mismatch_count -ne 0 -or
        $index.latest_restore.normal_project_unchanged -ne $true) {
        throw "Live recovery index did not retain the validated backup and restore results."
    }

    $failurePhase = "normal project preservation"
    Compare-LedgerSyncFinancialFingerprint -Before $normalBefore -After (Get-LedgerSyncFinancialFingerprint)
    if ($normalStateBefore -cne (Get-ComposeStateFingerprint)) {
        throw "Normal Compose service identity or health changed during live recovery acceptance."
    }
    $normalVolumesAfter = @(& docker volume ls `
        --filter "label=com.docker.compose.project=$script:LedgerSyncComposeProject" --format '{{.Name}}' | Sort-Object)
    if (($normalVolumesBefore -join "|") -cne ($normalVolumesAfter -join "|")) {
        throw "Normal Compose volume ownership changed during live recovery acceptance."
    }
    $remaining = Get-RestoreProjectResources -Project $restoreProject
    if ($remaining.Containers.Count -ne 0 -or $remaining.Volumes.Count -ne 0 -or $remaining.Networks.Count -ne 0) {
        throw "Live restore left isolated project resources behind."
    }
    Assert-CapturedOutputSanitized

    Write-Output "LIVE_ISOLATED_BACKUP_RESTORE=PASS"
    Write-Output "STREAMED_DUMP_AND_DIGEST=PASS"
    Write-Output "PROTECTED_HOST_FILES=PASS"
    Write-Output "SANITIZED_RECOVERY_INDEX=PASS"
    Write-Output "UNIQUE_RESTORE_PROJECT=PASS"
    Write-Output "NORMAL_FINANCIAL_STATE=UNCHANGED"
}
catch {
    $failure = $_
}
finally {
    $PSNativeCommandUseErrorActionPreference = $false
    try {
        if ($restoreProject -and $restoreProject -cmatch '^ledgersync-restore-\d{14}-[0-9a-f]{8}$') {
            $remaining = Get-RestoreProjectResources -Project $restoreProject
            if ($remaining.Containers.Count -gt 0 -or $remaining.Volumes.Count -gt 0 -or $remaining.Networks.Count -gt 0) {
                & docker compose -p $restoreProject -f $restoreComposeFile `
                    down --volumes --remove-orphans --timeout 10 *> $null
            }
        }
        Remove-RecoveryAcceptanceState
        if ($restoreProject) {
            $remaining = Get-RestoreProjectResources -Project $restoreProject
            if ($remaining.Containers.Count -ne 0 -or $remaining.Volumes.Count -ne 0 -or $remaining.Networks.Count -ne 0) {
                throw "Recovery acceptance cleanup left restore resources behind."
            }
        }
        if (Test-Path -LiteralPath $acceptanceState) {
            throw "Recovery acceptance cleanup left host state behind."
        }
        Write-Output "LIVE_RECOVERY_CLEANUP=PASS"
    }
    catch {
        $cleanupFailure = $_
    }
}

if ($null -ne $cleanupFailure) {
    throw "Live recovery acceptance cleanup failed; inspect only exact generated recovery resources."
}
if ($null -ne $failure) {
    throw "Live recovery acceptance failed during $failurePhase; captured output remained redacted."
}
