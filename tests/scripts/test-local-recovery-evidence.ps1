[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
. (Join-Path $repositoryRoot "scripts\local-runtime-common.ps1")
. (Join-Path $repositoryRoot "scripts\local-backup-common.ps1")

$temporaryParent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$testRoot = Join-Path $temporaryParent "ledgersync-recovery-evidence-$([Guid]::NewGuid().ToString('N'))"
$backupRoot = Join-Path $testRoot "backups"
$outsideRoot = Join-Path $testRoot "outside"
$junctionPath = $null
$bootstrapJunctionPath = $null
$originalRepositoryRoot = $script:LedgerSyncRepositoryRoot

function Assert-RecoveryTest {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}

function Assert-RecoveryRejected {
    param([scriptblock]$Action, [string]$Message)
    $rejected = $false
    try { & $Action }
    catch { $rejected = $true }
    Assert-RecoveryTest $rejected $Message
}

function New-TestBackupBundle {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [Parameter(Mandatory = $true)][DateTimeOffset]$CreatedAt,
        [Parameter(Mandatory = $true)][string]$Commit,
        [switch]$BadDigest,
        [switch]$MalformedJSON,
        [switch]$UnknownField
    )

    $leaf = "backup-$($CreatedAt.ToUniversalTime().ToString('yyyyMMddTHHmmssZ'))-$($Commit.Substring(0, 7))"
    $directory = Join-Path $Root $leaf
    New-Item -ItemType Directory -Path $directory | Out-Null
    $dumpPath = Join-Path $directory "database.dump"
    [IO.File]::WriteAllBytes($dumpPath, [byte[]](1, 3, 3, 7, 9, 11, 13, 17))
    if ($MalformedJSON) {
        [IO.File]::WriteAllText((Join-Path $directory "manifest.json"), "{not-json", [Text.UTF8Encoding]::new($false))
        return $directory
    }
    $digest = (Get-FileHash -LiteralPath $dumpPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($BadDigest) { $digest = "0" * 64 }
    $manifest = [ordered]@{
        format_version = $script:LedgerSyncBackupFormatVersion
        created_at_utc = $CreatedAt.ToUniversalTime().ToString("o")
        source_commit = $Commit
        scope = [ordered]@{
            deployment = "local-only"
            database = "all local LedgerSync tenants"
            manifest_data = "redacted counts and integrity metadata only"
            currency = "INR"
        }
        schema = [ordered]@{ migration_version = "000015_operations_read_models.up.sql"; migration_count = 15 }
        database = [ordered]@{ file_name = "database.dump"; byte_length = 8; sha256 = $digest }
        counts = [ordered]@{ accounts = 6; transfers = 10; ledger_postings = 20 }
    }
    if ($UnknownField) { $manifest["unexpected"] = "rejected" }
    [IO.File]::WriteAllText(
        (Join-Path $directory "manifest.json"),
        ($manifest | ConvertTo-Json -Depth 6),
        [Text.UTF8Encoding]::new($false)
    )
    return $directory
}

try {
    New-Item -ItemType Directory -Path $backupRoot -Force | Out-Null
    New-Item -ItemType Directory -Path $outsideRoot -Force | Out-Null
    $baseTime = [DateTimeOffset]::Parse("2026-08-25T01:00:00Z")
    $oldest = New-TestBackupBundle -Root $backupRoot -CreatedAt $baseTime -Commit ("1" * 40)
    $middle = New-TestBackupBundle -Root $backupRoot -CreatedAt $baseTime.AddMinutes(1) -Commit ("2" * 40)
    $newest = New-TestBackupBundle -Root $backupRoot -CreatedAt $baseTime.AddMinutes(2) -Commit ("3" * 40)

    $validated = Assert-LedgerSyncBackupBundle -BackupDirectory $newest -BackupRoot $backupRoot
    Assert-RecoveryTest ($validated.Manifest.source_commit -ceq ("3" * 40)) "A canonical finalized backup did not validate."

    $outsideBundle = New-TestBackupBundle -Root $outsideRoot -CreatedAt $baseTime.AddMinutes(3) -Commit ("4" * 40)
    Assert-RecoveryRejected {
        Assert-LedgerSyncBackupBundle -BackupDirectory $outsideBundle -BackupRoot $backupRoot | Out-Null
    } "A backup outside its configured root was accepted."
    $nestedRoot = Join-Path $backupRoot "nested"
    New-Item -ItemType Directory -Path $nestedRoot | Out-Null
    $nestedBundle = New-TestBackupBundle -Root $nestedRoot -CreatedAt $baseTime.AddMinutes(7) -Commit ("9" * 40)
    $traversalPath = Join-Path $backupRoot "nested\..\nested\$(Split-Path -Leaf $nestedBundle)"
    Assert-RecoveryRejected {
        Assert-LedgerSyncBackupBundle -BackupDirectory $traversalPath -BackupRoot $backupRoot | Out-Null
    } "A traversal/nested backup path was accepted."

    $malformed = New-TestBackupBundle -Root $backupRoot -CreatedAt $baseTime.AddMinutes(4) -Commit ("5" * 40) -MalformedJSON
    $badDigest = New-TestBackupBundle -Root $backupRoot -CreatedAt $baseTime.AddMinutes(5) -Commit ("6" * 40) -BadDigest
    $unknown = New-TestBackupBundle -Root $backupRoot -CreatedAt $baseTime.AddMinutes(6) -Commit ("7" * 40) -UnknownField
    New-Item -ItemType Directory -Path (Join-Path $backupRoot ".partial-$([Guid]::NewGuid().ToString('N'))") | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $backupRoot ".recovery-evidence-index.pending-$([Guid]::NewGuid().ToString('N'))") | Out-Null

    Assert-RecoveryRejected {
        Assert-LedgerSyncBackupBundle -BackupDirectory $malformed -BackupRoot $backupRoot | Out-Null
    } "Malformed manifest JSON was accepted."
    Assert-RecoveryRejected {
        Assert-LedgerSyncBackupBundle -BackupDirectory $badDigest -BackupRoot $backupRoot | Out-Null
    } "A digest mismatch was accepted."
    Assert-RecoveryRejected {
        Assert-LedgerSyncBackupBundle -BackupDirectory $unknown -BackupRoot $backupRoot | Out-Null
    } "An unknown manifest field was accepted."

    $futureMalformedLeaf = "backup-99999999T999999Z-abcdef0"
    $futureMalformed = Join-Path $backupRoot $futureMalformedLeaf
    New-Item -ItemType Directory -Path $futureMalformed | Out-Null
    [IO.File]::WriteAllText((Join-Path $futureMalformed "manifest.json"), "{}", [Text.UTF8Encoding]::new($false))

    $retained = Invoke-LedgerSyncBackupRetention -BackupRoot $backupRoot -RetentionCount 1
    Assert-RecoveryTest (Test-Path -LiteralPath $newest -PathType Container) "Retention deleted the newest validated backup."
    Assert-RecoveryTest (-not (Test-Path -LiteralPath $oldest)) "Retention kept the oldest validated backup."
    Assert-RecoveryTest (-not (Test-Path -LiteralPath $middle)) "Retention kept an expired validated backup."
    Assert-RecoveryTest (Test-Path -LiteralPath $futureMalformed) "Retention deleted a malformed future-timestamp directory."
    Assert-RecoveryTest (@($retained.Bundles).Count -eq 1) "Retention returned an incorrect validated backup count."

    $receipt = Write-LedgerSyncRestoreEvidence -Backup $validated -ReconciliationStatus "matched" -LocalRTOSeconds 4.25
    Assert-RecoveryTest ($receipt.Evidence.normal_project_unchanged -eq $true) "Restore evidence omitted normal-project preservation."
    $index = Write-LedgerSyncRecoveryEvidenceIndex -BackupRoot $backupRoot -RetentionCount 1
    Assert-RecoveryTest ($index.latest_backup.backup_id -ceq (Split-Path -Leaf $newest)) "Recovery index selected the wrong latest valid backup."
    Assert-RecoveryTest ($index.latest_restore.backup_id -ceq (Split-Path -Leaf $newest)) "Recovery index omitted the validated restore receipt."
    Assert-RecoveryTest ([int]$index.retention.valid_backup_count -eq 1) "Recovery index exposed an invalid backup as valid."
    Assert-RecoveryTest ([int]$index.retention.ignored_entry_count -ge 5) "Recovery index did not ignore hostile/incomplete entries."
    $indexPath = Join-Path $backupRoot $script:LedgerSyncRecoveryEvidenceFileName
    $indexText = Get-Content -LiteralPath $indexPath -Raw
    Assert-RecoveryTest ($indexText.Length -lt 65536) "Recovery evidence index is not bounded."
    Assert-RecoveryTest ($indexText -notmatch '(?i)(database\.dump|sha256\s*":\s*"[a-f0-9]{64}|password|secret|token)') "Recovery index exposed prohibited recovery material."
    Assert-RecoveryTest (-not $indexText.Contains($backupRoot, [StringComparison]::OrdinalIgnoreCase)) "Recovery index exposed its host root."
    if ($env:OS -eq "Windows_NT") {
        Assert-RecoveryTest ((Get-Acl -LiteralPath $indexPath).AreAccessRulesProtected) "Recovery evidence index ACL is not protected."
    }

    $tampered = $index | ConvertTo-Json -Depth 8 | ConvertFrom-Json -AsHashtable
    $tampered["unexpected"] = "reject"
    [IO.File]::WriteAllText($indexPath, ($tampered | ConvertTo-Json -Depth 8), [Text.UTF8Encoding]::new($false))
    Assert-RecoveryRejected {
        Read-LedgerSyncRecoveryEvidenceIndex -BackupRoot $backupRoot | Out-Null
    } "An unknown recovery-index field was accepted."
    Write-LedgerSyncRecoveryEvidenceIndex -BackupRoot $backupRoot -RetentionCount 1 | Out-Null

    $bootstrapRepository = Join-Path $testRoot "bootstrap-repository"
    New-Item -ItemType Directory -Path $bootstrapRepository | Out-Null
    $script:LedgerSyncRepositoryRoot = $bootstrapRepository
    $emptyIndex = Initialize-LedgerSyncLocalRecoveryEvidenceIndex
    $bootstrapIndexPath = Join-Path $bootstrapRepository "data\local-backups\recovery-evidence-index.json"
    Assert-RecoveryTest (Test-Path -LiteralPath $bootstrapIndexPath -PathType Leaf) "Startup bootstrap did not create the exact recovery index file."
    Assert-RecoveryTest ($null -eq $emptyIndex.latest_backup -and $null -eq $emptyIndex.latest_restore -and
        [int]$emptyIndex.retention.valid_backup_count -eq 0) "Startup bootstrap index was not an empty v1 summary."
    $bootstrapDigest = (Get-FileHash -LiteralPath $bootstrapIndexPath -Algorithm SHA256).Hash
    $bootstrapWriteTime = (Get-Item -LiteralPath $bootstrapIndexPath).LastWriteTimeUtc
    Start-Sleep -Milliseconds 20
    Initialize-LedgerSyncLocalRecoveryEvidenceIndex | Out-Null
    Assert-RecoveryTest ((Get-FileHash -LiteralPath $bootstrapIndexPath -Algorithm SHA256).Hash -ceq $bootstrapDigest) "Startup bootstrap overwrote an existing valid index."
    Assert-RecoveryTest ((Get-Item -LiteralPath $bootstrapIndexPath).LastWriteTimeUtc -eq $bootstrapWriteTime) "Startup bootstrap rewrote an existing valid index."

    [IO.File]::WriteAllText($bootstrapIndexPath, "{malformed", [Text.UTF8Encoding]::new($false))
    Protect-LedgerSyncRecoveryFile -Path $bootstrapIndexPath
    Assert-RecoveryRejected {
        Initialize-LedgerSyncLocalRecoveryEvidenceIndex | Out-Null
    } "Startup bootstrap accepted or replaced a malformed existing index."
    Assert-RecoveryTest ((Get-Content -LiteralPath $bootstrapIndexPath -Raw) -ceq "{malformed") "Startup bootstrap overwrote malformed operator evidence."

    $junctionBootstrapRepository = Join-Path $testRoot "bootstrap-junction-repository"
    $junctionBootstrapTarget = Join-Path $outsideRoot "bootstrap-data-target"
    New-Item -ItemType Directory -Path $junctionBootstrapRepository | Out-Null
    New-Item -ItemType Directory -Path $junctionBootstrapTarget | Out-Null
    $bootstrapJunctionPath = Join-Path $junctionBootstrapRepository "data"
    if ($env:OS -eq "Windows_NT") {
        New-Item -ItemType Junction -Path $bootstrapJunctionPath -Target $junctionBootstrapTarget | Out-Null
    } else {
        New-Item -ItemType SymbolicLink -Path $bootstrapJunctionPath -Target $junctionBootstrapTarget | Out-Null
    }
    $script:LedgerSyncRepositoryRoot = $junctionBootstrapRepository
    Assert-RecoveryRejected {
        Initialize-LedgerSyncLocalRecoveryEvidenceIndex | Out-Null
    } "Startup bootstrap traversed a reparse-point ancestor."
    Assert-RecoveryTest (-not (Test-Path -LiteralPath (Join-Path $junctionBootstrapTarget "local-backups"))) "Startup bootstrap created state beyond a reparse-point ancestor."
    Remove-Item -LiteralPath $bootstrapJunctionPath -Force
    $bootstrapJunctionPath = $null
    $script:LedgerSyncRepositoryRoot = $originalRepositoryRoot

    $junctionTarget = Join-Path $outsideRoot "junction-target"
    New-Item -ItemType Directory -Path $junctionTarget | Out-Null
    $junctionLeaf = "backup-20260825T020000Z-8888888"
    $junctionPath = Join-Path $backupRoot $junctionLeaf
    if ($env:OS -eq "Windows_NT") {
        New-Item -ItemType Junction -Path $junctionPath -Target $junctionTarget | Out-Null
    } else {
        New-Item -ItemType SymbolicLink -Path $junctionPath -Target $junctionTarget | Out-Null
    }
    Assert-RecoveryRejected {
        Assert-LedgerSyncCanonicalBackupChild -BackupRoot $backupRoot -BackupDirectory $junctionPath | Out-Null
    } "A symlink/reparse-point backup directory was accepted."
    Remove-Item -LiteralPath $junctionPath -Force
    $junctionPath = $null

    Write-Output "LOCAL_RECOVERY_EVIDENCE_TESTS=PASS"
    Write-Output "PATH_TRAVERSAL=REJECTED"
    Write-Output "REPARSE_POINT=REJECTED"
    Write-Output "MALFORMED_INCOMPLETE_DIGEST=IGNORED"
    Write-Output "RETENTION_NEWEST_VALID=PRESERVED"
    Write-Output "SANITIZED_BOUNDED_INDEX=PASS"
    Write-Output "STARTUP_EMPTY_INDEX_FAIL_CLOSED=PASS"
}
finally {
    $script:LedgerSyncRepositoryRoot = $originalRepositoryRoot
    if ($bootstrapJunctionPath -and (Test-Path -LiteralPath $bootstrapJunctionPath)) {
        Remove-Item -LiteralPath $bootstrapJunctionPath -Force
    }
    if ($junctionPath -and (Test-Path -LiteralPath $junctionPath)) {
        Remove-Item -LiteralPath $junctionPath -Force
    }
    if (Test-Path -LiteralPath $testRoot) {
        Remove-LedgerSyncValidatedDirectory -Parent $temporaryParent -Directory $testRoot `
            -AllowedLeafPattern '^ledgersync-recovery-evidence-[0-9a-f]{32}$'
    }
}
