Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$script:LedgerSyncBackupFormatVersion = "ledgersync-local-backup/v1"
$script:LedgerSyncBackupDirectoryPattern = '^backup-\d{8}T\d{6}Z-[0-9a-f]{7,40}$'
$script:LedgerSyncRecoveryEvidenceFormatVersion = "ledgersync-recovery-evidence-index/v1"
$script:LedgerSyncRestoreEvidenceFormatVersion = "ledgersync-local-restore-evidence/v1"
$script:LedgerSyncRecoveryEvidenceFileName = "recovery-evidence-index.json"
$script:LedgerSyncRestoreEvidenceFileName = "restore-evidence.json"
$script:LedgerSyncMaximumBackupEntries = 200

function Invoke-LedgerSyncFileToContainerCommand {
    param(
        [Parameter(Mandatory = $true)][string]$SourcePath,
        [Parameter(Mandatory = $true)][string]$ContainerID,
        [Parameter(Mandatory = $true)][string[]]$CommandArguments
    )

    $resolvedSource = [IO.Path]::GetFullPath($SourcePath)
    if (-not (Test-Path -LiteralPath $resolvedSource -PathType Leaf)) {
        throw "The bounded container-copy source does not exist."
    }
    if ($ContainerID -cnotmatch '^[0-9a-f]{12,64}$') {
        throw "The bounded container-copy target is not an exact container ID."
    }
    if ($CommandArguments.Count -lt 1 -or @($CommandArguments | Where-Object { [string]::IsNullOrWhiteSpace($_) }).Count -ne 0) {
        throw "The bounded container command is empty or malformed."
    }

    # `docker cp` rejects writes to containers whose root filesystem is marked
    # read-only, even when the destination is a writable tmpfs. Stream through
    # the container process instead; byte copying avoids PowerShell text
    # transcoding and keeps the hardened rootfs contract intact.
    $startInfo = [Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = "docker"
    $startInfo.UseShellExecute = $false
    $startInfo.RedirectStandardInput = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    foreach ($argument in @("exec", "-i", $ContainerID) + $CommandArguments) {
        [void]$startInfo.ArgumentList.Add($argument)
    }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $startInfo
    $source = $null
    try {
        if (-not $process.Start()) { throw "Could not start the bounded container copy." }
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        $source = [IO.File]::OpenRead($resolvedSource)
        $source.CopyTo($process.StandardInput.BaseStream)
        $process.StandardInput.Close()
        $process.WaitForExit()
        $stdoutTask.GetAwaiter().GetResult() | Out-Null
        $stderr = $stderrTask.GetAwaiter().GetResult()
        if ($process.ExitCode -ne 0) {
            throw "The bounded container copy failed: $($stderr.Trim())"
        }
    }
    finally {
        if ($null -ne $source) { $source.Dispose() }
        $process.Dispose()
    }
}

function Copy-LedgerSyncFileToContainer {
    param(
        [Parameter(Mandatory = $true)][string]$SourcePath,
        [Parameter(Mandatory = $true)][string]$ContainerID,
        [Parameter(Mandatory = $true)][string]$ContainerPath
    )
    if ($ContainerPath -cnotmatch '^/tmp/[a-zA-Z0-9._-]+$') {
        throw "The bounded container-copy destination is not an approved /tmp path."
    }
    Invoke-LedgerSyncFileToContainerCommand -SourcePath $SourcePath -ContainerID $ContainerID `
        -CommandArguments @("sh", "-c", "cat > '$ContainerPath'")
}

function Invoke-LedgerSyncContainerCommandToFile {
    param(
        [Parameter(Mandatory = $true)][string]$ContainerID,
        [Parameter(Mandatory = $true)][string[]]$CommandArguments,
        [Parameter(Mandatory = $true)][string]$DestinationPath
    )
    if ($ContainerID -cnotmatch '^[0-9a-f]{12,64}$') {
        throw "The bounded container-stream source is not an exact container ID."
    }
    if ($CommandArguments.Count -lt 1 -or @($CommandArguments | Where-Object { [string]::IsNullOrWhiteSpace($_) }).Count -ne 0) {
        throw "The bounded container-stream command is empty or malformed."
    }
    $resolvedDestination = [IO.Path]::GetFullPath($DestinationPath)
    $destinationParent = Split-Path -Parent $resolvedDestination
    if (-not (Test-Path -LiteralPath $destinationParent -PathType Container)) {
        throw "The bounded container-stream destination parent does not exist."
    }

    $startInfo = [Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = "docker"
    $startInfo.UseShellExecute = $false
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    foreach ($argument in @("exec", $ContainerID) + $CommandArguments) {
        [void]$startInfo.ArgumentList.Add($argument)
    }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $startInfo
    $destination = $null
    try {
        if (-not $process.Start()) { throw "Could not start the bounded container stream." }
        $stderrTask = $process.StandardError.ReadToEndAsync()
        $destination = [IO.File]::Open($resolvedDestination, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
        $process.StandardOutput.BaseStream.CopyTo($destination)
        $destination.Flush($true)
        $destination.Dispose()
        $destination = $null
        $process.WaitForExit()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        if ($process.ExitCode -ne 0) {
            Remove-Item -LiteralPath $resolvedDestination -Force -ErrorAction SilentlyContinue
            throw "The bounded container stream failed: $($stderr.Trim())"
        }
    }
    finally {
        if ($null -ne $destination) { $destination.Dispose() }
        $process.Dispose()
    }
}

function Resolve-LedgerSyncBackupRoot {
    param([string]$BackupRoot)

    if ([string]::IsNullOrWhiteSpace($BackupRoot)) {
        $BackupRoot = Join-Path $script:LedgerSyncRepositoryRoot "data\local-backups"
    }
    elseif (-not [IO.Path]::IsPathRooted($BackupRoot)) {
        $BackupRoot = Join-Path $script:LedgerSyncRepositoryRoot $BackupRoot
    }

    $resolvedRoot = [IO.Path]::GetFullPath($BackupRoot).TrimEnd(
        [IO.Path]::DirectorySeparatorChar,
        [IO.Path]::AltDirectorySeparatorChar
    )
    $repositoryRoot = [IO.Path]::GetFullPath($script:LedgerSyncRepositoryRoot).TrimEnd(
        [IO.Path]::DirectorySeparatorChar,
        [IO.Path]::AltDirectorySeparatorChar
    )
    $fileSystemRoot = [IO.Path]::GetPathRoot($resolvedRoot).TrimEnd(
        [IO.Path]::DirectorySeparatorChar,
        [IO.Path]::AltDirectorySeparatorChar
    )
    if ($resolvedRoot -eq $repositoryRoot -or $resolvedRoot -eq $fileSystemRoot) {
        throw "Backup root must be a dedicated directory, not the repository or filesystem root."
    }
    return $resolvedRoot
}

function Test-LedgerSyncExactChildPath {
    param(
        [Parameter(Mandatory = $true)][string]$Parent,
        [Parameter(Mandatory = $true)][string]$Child
    )

    $resolvedParent = [IO.Path]::GetFullPath($Parent).TrimEnd(
        [IO.Path]::DirectorySeparatorChar,
        [IO.Path]::AltDirectorySeparatorChar
    )
    $resolvedChild = [IO.Path]::GetFullPath($Child)
    $prefix = $resolvedParent + [IO.Path]::DirectorySeparatorChar
    return $resolvedChild.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)
}

function Assert-LedgerSyncNoReparsePoints {
    param([Parameter(Mandatory = $true)][string]$Path)

    $resolved = [IO.Path]::GetFullPath($Path)
    if (-not (Test-Path -LiteralPath $resolved)) {
        throw "The protected recovery path does not exist."
    }
    $item = Get-Item -LiteralPath $resolved -Force
    if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Protected recovery paths cannot contain a symlink or reparse point."
    }
    $current = if ($item.PSIsContainer) { $item } else { $item.Directory }
    while ($null -ne $current) {
        if (($current.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "Protected recovery paths cannot contain a symlink or reparse point."
        }
        $parent = $current.Parent
        if ($null -eq $parent -or $parent.FullName -eq $current.FullName) { break }
        $current = $parent
    }
}

function Assert-LedgerSyncProspectivePathNoReparsePoints {
    param([Parameter(Mandatory = $true)][string]$Path)

    $current = [IO.Path]::GetFullPath($Path)
    while (-not (Test-Path -LiteralPath $current)) {
        $parent = [IO.Directory]::GetParent($current)
        if ($null -eq $parent -or $parent.FullName -eq $current) {
            throw "Could not resolve a safe existing ancestor for the recovery path."
        }
        $current = $parent.FullName
    }
    Assert-LedgerSyncNoReparsePoints -Path $current
}

function Assert-LedgerSyncCanonicalBackupChild {
    param(
        [Parameter(Mandatory = $true)][string]$BackupRoot,
        [Parameter(Mandatory = $true)][string]$BackupDirectory
    )

    $resolvedRoot = Resolve-LedgerSyncBackupRoot -BackupRoot $BackupRoot
    $resolvedDirectory = [IO.Path]::GetFullPath($BackupDirectory).TrimEnd(
        [IO.Path]::DirectorySeparatorChar,
        [IO.Path]::AltDirectorySeparatorChar
    )
    if (-not (Test-Path -LiteralPath $resolvedRoot -PathType Container) -or
        -not (Test-Path -LiteralPath $resolvedDirectory -PathType Container)) {
        throw "The protected backup root or bundle does not exist."
    }
    Assert-LedgerSyncNoReparsePoints -Path $resolvedRoot
    Assert-LedgerSyncNoReparsePoints -Path $resolvedDirectory
    $parent = [IO.Directory]::GetParent($resolvedDirectory)
    if ($null -eq $parent -or -not $parent.FullName.TrimEnd(
            [IO.Path]::DirectorySeparatorChar,
            [IO.Path]::AltDirectorySeparatorChar
        ).Equals($resolvedRoot, [StringComparison]::OrdinalIgnoreCase)) {
        throw "The backup bundle must be an immediate child of its configured root."
    }
    $leaf = [IO.Path]::GetFileName($resolvedDirectory)
    if ($leaf -cnotmatch $script:LedgerSyncBackupDirectoryPattern) {
        throw "The backup bundle name is not finalized or supported."
    }
    return $resolvedDirectory
}

function Remove-LedgerSyncValidatedDirectory {
    param(
        [Parameter(Mandatory = $true)][string]$Parent,
        [Parameter(Mandatory = $true)][string]$Directory,
        [Parameter(Mandatory = $true)][string]$AllowedLeafPattern
    )

    $resolvedParent = [IO.Path]::GetFullPath($Parent)
    $resolvedDirectory = [IO.Path]::GetFullPath($Directory)
    $leaf = Split-Path -Leaf $resolvedDirectory
    if (-not (Test-LedgerSyncExactChildPath -Parent $resolvedParent -Child $resolvedDirectory)) {
        throw "Refusing to remove a directory outside the validated parent: $resolvedDirectory"
    }
    if ($leaf -cnotmatch $AllowedLeafPattern) {
        throw "Refusing to remove an unexpected directory name: $leaf"
    }
    if (Test-Path -LiteralPath $resolvedDirectory) {
        $item = Get-Item -LiteralPath $resolvedDirectory -Force
        if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "Refusing to recursively remove a reparse-point directory: $resolvedDirectory"
        }
        Remove-Item -LiteralPath $resolvedDirectory -Recurse -Force
    }
}

function Assert-LedgerSyncJsonObjectProperties {
    param(
        [Parameter(Mandatory = $true)][Text.Json.JsonElement]$Element,
        [Parameter(Mandatory = $true)][string[]]$Expected,
        [Parameter(Mandatory = $true)][string]$ObjectName
    )

    if ($Element.ValueKind -ne [Text.Json.JsonValueKind]::Object) {
        throw "$ObjectName must be a JSON object."
    }
    $actual = @($Element.EnumerateObject() | ForEach-Object { $_.Name })
    $unique = @($actual | Sort-Object -Unique)
    if ($actual.Count -ne $Expected.Count -or $unique.Count -ne $Expected.Count -or
        @($Expected | Where-Object { $unique -cnotcontains $_ }).Count -ne 0) {
        throw "$ObjectName contains missing, duplicate, or unsupported fields."
    }
}

function Assert-LedgerSyncBackupBundle {
    param(
        [Parameter(Mandatory = $true)][string]$BackupDirectory,
        [Parameter(Mandatory = $true)][string]$BackupRoot
    )

    $resolvedDirectory = Assert-LedgerSyncCanonicalBackupChild `
        -BackupRoot $BackupRoot -BackupDirectory $BackupDirectory

    $manifestPath = Join-Path $resolvedDirectory "manifest.json"
    if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
        throw "Backup manifest is missing."
    }
    Assert-LedgerSyncNoReparsePoints -Path $manifestPath
    if ((Get-Item -LiteralPath $manifestPath).Length -gt 16384) {
        throw "Backup manifest exceeds the bounded schema size."
    }

    try {
        $manifestJson = Get-Content -LiteralPath $manifestPath -Raw
        $manifest = $manifestJson | ConvertFrom-Json
        $jsonDocument = [Text.Json.JsonDocument]::Parse($manifestJson)
        try {
            $rootElement = $jsonDocument.RootElement
            Assert-LedgerSyncJsonObjectProperties -Element $rootElement `
                -Expected @("format_version", "created_at_utc", "source_commit", "scope", "schema", "database", "counts") `
                -ObjectName "Backup manifest"
            $scopeElement = $rootElement.GetProperty("scope")
            $schemaElement = $rootElement.GetProperty("schema")
            $databaseElement = $rootElement.GetProperty("database")
            $countsElement = $rootElement.GetProperty("counts")
            Assert-LedgerSyncJsonObjectProperties -Element $scopeElement `
                -Expected @("deployment", "database", "manifest_data", "currency") -ObjectName "Backup scope"
            Assert-LedgerSyncJsonObjectProperties -Element $schemaElement `
                -Expected @("migration_version", "migration_count") -ObjectName "Backup schema"
            Assert-LedgerSyncJsonObjectProperties -Element $databaseElement `
                -Expected @("file_name", "byte_length", "sha256") -ObjectName "Backup database metadata"
            Assert-LedgerSyncJsonObjectProperties -Element $countsElement `
                -Expected @("accounts", "transfers", "ledger_postings") -ObjectName "Backup counts"
            foreach ($numberElement in @(
                $schemaElement.GetProperty("migration_count"),
                $databaseElement.GetProperty("byte_length"),
                $countsElement.GetProperty("accounts"),
                $countsElement.GetProperty("transfers"),
                $countsElement.GetProperty("ledger_postings")
            )) {
                if ($numberElement.ValueKind -ne [Text.Json.JsonValueKind]::Number) {
                    throw "Backup manifest numeric fields must be JSON numbers."
                }
            }
            $createdAtText = $rootElement.GetProperty("created_at_utc").GetString()
        }
        finally {
            $jsonDocument.Dispose()
        }
    }
    catch {
        throw "Backup manifest is not valid JSON."
    }

    if ($manifest.format_version -cne $script:LedgerSyncBackupFormatVersion) {
        throw "Unsupported backup format '$($manifest.format_version)'."
    }
    $createdAt = [DateTimeOffset]::MinValue
    if (-not [DateTimeOffset]::TryParse($createdAtText, [Globalization.CultureInfo]::InvariantCulture, [Globalization.DateTimeStyles]::RoundtripKind, [ref]$createdAt) -or $createdAt.Offset -ne [TimeSpan]::Zero) {
        throw "Backup manifest creation time is missing or is not UTC."
    }
    if ([string]$manifest.source_commit -cnotmatch '^[0-9a-f]{40}$') {
        throw "Backup manifest source commit is missing or malformed."
    }
    if ($manifest.scope.deployment -cne "local-only" -or
        $manifest.scope.database -cne "all local LedgerSync tenants" -or
        $manifest.scope.manifest_data -cne "redacted counts and integrity metadata only" -or
        $manifest.scope.currency -cne "INR") {
        throw "Backup manifest scope is missing or is not the supported local-only boundary."
    }
    if ($manifest.database.file_name -cne "database.dump") {
        throw "Backup manifest references an unexpected database file."
    }

    $dumpPath = Join-Path $resolvedDirectory ([string]$manifest.database.file_name)
    if (-not (Test-Path -LiteralPath $dumpPath -PathType Leaf)) {
        throw "Backup database dump is missing."
    }
    Assert-LedgerSyncNoReparsePoints -Path $dumpPath

    $dump = Get-Item -LiteralPath $dumpPath
    if ([int64]$manifest.database.byte_length -ne [int64]$dump.Length) {
        throw "Backup byte length does not match the recorded manifest."
    }
    $actualDigest = (Get-FileHash -LiteralPath $dumpPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ([string]$manifest.database.sha256 -cnotmatch '^[0-9a-f]{64}$') {
        throw "Backup manifest SHA-256 is missing or malformed."
    }
    if ([string]$manifest.database.sha256 -cne $actualDigest) {
        throw "Backup SHA-256 digest does not match the recorded manifest."
    }

    foreach ($requiredCount in @("accounts", "transfers", "ledger_postings")) {
        if ($null -eq $manifest.counts.$requiredCount -or [int64]$manifest.counts.$requiredCount -lt 0) {
            throw "Backup manifest has an invalid '$requiredCount' count."
        }
    }
    if ([string]$manifest.schema.migration_version -cnotmatch '^[0-9]{6}_[a-z0-9._-]{1,120}$') {
        throw "Backup manifest has no migration version."
    }
    if ([int64]$manifest.schema.migration_count -lt 1) {
        throw "Backup manifest has an invalid migration count."
    }

    $expectedLeaf = "backup-$($createdAt.ToUniversalTime().ToString('yyyyMMddTHHmmssZ'))-$(([string]$manifest.source_commit).Substring(0, 7))"
    if ((Split-Path -Leaf $resolvedDirectory) -cne $expectedLeaf) {
        throw "Backup bundle identity does not match its bound timestamp and source commit."
    }

    return [pscustomobject]@{
        Directory = $resolvedDirectory
        DumpPath = $dumpPath
        ManifestPath = $manifestPath
        Manifest = $manifest
        CreatedAt = $createdAt
    }
}

function Test-LedgerSyncBackupCorruptionGuard {
    param(
        [Parameter(Mandatory = $true)][string]$BackupDirectory,
        [Parameter(Mandatory = $true)][string]$BackupRoot
    )

    $source = Assert-LedgerSyncBackupBundle `
        -BackupDirectory $BackupDirectory -BackupRoot $BackupRoot
    $suffix = [Guid]::NewGuid().ToString("N")
    $temporaryParent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    $temporaryRoot = Join-Path $temporaryParent "ledgersync-corruption-$suffix"
    $temporaryDirectory = Join-Path $temporaryRoot (Split-Path -Leaf $source.Directory)
    try {
        New-Item -ItemType Directory -Path $temporaryRoot | Out-Null
        Copy-Item -LiteralPath $source.Directory -Destination $temporaryDirectory -Recurse
        $mutatedDump = Join-Path $temporaryDirectory "database.dump"
        $stream = [IO.File]::Open($mutatedDump, [IO.FileMode]::Open, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
        try {
            if ($stream.Length -lt 1) {
                throw "Cannot test corruption detection against an empty dump."
            }
            $offset = [Math]::Floor($stream.Length / 2)
            $stream.Position = $offset
            $original = $stream.ReadByte()
            $stream.Position = $offset
            $stream.WriteByte([byte]($original -bxor 0x01))
        }
        finally {
            $stream.Dispose()
        }

        $rejected = $false
        try {
            Assert-LedgerSyncBackupBundle `
                -BackupDirectory $temporaryDirectory -BackupRoot $temporaryRoot | Out-Null
        }
        catch {
            if ($_.Exception.Message -notmatch "SHA-256 digest") {
                throw
            }
            $rejected = $true
        }
        if (-not $rejected) {
            throw "Corrupted backup unexpectedly passed digest validation."
        }
        return $true
    }
    finally {
        if (Test-Path -LiteralPath $temporaryRoot) {
            Remove-LedgerSyncValidatedDirectory `
                -Parent $temporaryParent `
                -Directory $temporaryRoot `
                -AllowedLeafPattern '^ledgersync-corruption-[0-9a-f]{32}$'
        }
    }
}

function Protect-LedgerSyncRecoveryFile {
    param([Parameter(Mandatory = $true)][string]$Path)

    Protect-LedgerSyncLocalSecretFile -Path $Path
    if ($env:OS -eq "Windows_NT") {
        $acl = Get-Acl -LiteralPath $Path
        if (-not $acl.AreAccessRulesProtected) {
            throw "A protected recovery evidence file did not retain its restricted ACL."
        }
    }
}

function Get-LedgerSyncValidatedBackupSet {
    param([Parameter(Mandatory = $true)][string]$BackupRoot)

    $resolvedRoot = Resolve-LedgerSyncBackupRoot -BackupRoot $BackupRoot
    if (-not (Test-Path -LiteralPath $resolvedRoot -PathType Container)) {
        return [pscustomobject]@{ Root = $resolvedRoot; Bundles = @(); IgnoredCount = 0 }
    }
    Assert-LedgerSyncNoReparsePoints -Path $resolvedRoot
    $entries = @(Get-ChildItem -LiteralPath $resolvedRoot -Directory -Force | Select-Object -First ($script:LedgerSyncMaximumBackupEntries + 1))
    if ($entries.Count -gt $script:LedgerSyncMaximumBackupEntries) {
        throw "Backup root contains more entries than the bounded recovery index permits."
    }
    $bundles = @()
    $ignored = 0
    foreach ($entry in $entries) {
        if ($entry.Name -cnotmatch $script:LedgerSyncBackupDirectoryPattern) {
            $ignored++
            continue
        }
        try {
            $bundle = Assert-LedgerSyncBackupBundle `
                -BackupDirectory $entry.FullName -BackupRoot $resolvedRoot
            $bundles += [pscustomobject]@{ Bundle = $bundle; CreatedAt = $bundle.CreatedAt }
        }
        catch {
            $ignored++
        }
    }
    return [pscustomobject]@{
        Root = $resolvedRoot
        Bundles = @($bundles | Sort-Object CreatedAt -Descending)
        IgnoredCount = $ignored
    }
}

function Invoke-LedgerSyncBackupRetention {
    param(
        [Parameter(Mandatory = $true)][string]$BackupRoot,
        [ValidateRange(1, 100)][int]$RetentionCount
    )

    $set = Get-LedgerSyncValidatedBackupSet -BackupRoot $BackupRoot
    $newest = @($set.Bundles | Select-Object -First 1)
    foreach ($expired in @($set.Bundles | Select-Object -Skip $RetentionCount)) {
        if ($newest.Count -eq 1 -and
            $expired.Bundle.Directory.Equals($newest[0].Bundle.Directory, [StringComparison]::OrdinalIgnoreCase)) {
            throw "Retention refused to remove the newest validated backup."
        }
        # Revalidate immediately before deletion so a directory cannot be
        # swapped after enumeration and still become a recursive delete target.
        Assert-LedgerSyncBackupBundle `
            -BackupDirectory $expired.Bundle.Directory -BackupRoot $set.Root | Out-Null
        Remove-LedgerSyncValidatedDirectory `
            -Parent $set.Root -Directory $expired.Bundle.Directory `
            -AllowedLeafPattern $script:LedgerSyncBackupDirectoryPattern
    }
    return Get-LedgerSyncValidatedBackupSet -BackupRoot $set.Root
}

function Read-LedgerSyncRestoreEvidence {
    param(
        [Parameter(Mandatory = $true)][object]$Backup,
        [switch]$AllowMissing
    )

    $path = Join-Path $Backup.Directory $script:LedgerSyncRestoreEvidenceFileName
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        if ($AllowMissing) { return $null }
        throw "Restore evidence is missing."
    }
    Assert-LedgerSyncNoReparsePoints -Path $path
    if ($env:OS -eq "Windows_NT" -and -not (Get-Acl -LiteralPath $path).AreAccessRulesProtected) {
        throw "Restore evidence is not protected for local read access."
    }
    if ((Get-Item -LiteralPath $path).Length -gt 8192) {
        throw "Restore evidence exceeds the bounded schema size."
    }
    try {
        $json = Get-Content -LiteralPath $path -Raw
        $document = [Text.Json.JsonDocument]::Parse($json)
        try {
            Assert-LedgerSyncJsonObjectProperties -Element $document.RootElement `
                -Expected @("format_version", "backup_id", "completed_at_utc", "status", "reconciliation", "normal_project_unchanged", "local_rto_seconds") `
                -ObjectName "Restore evidence"
            Assert-LedgerSyncJsonObjectProperties -Element $document.RootElement.GetProperty("reconciliation") `
                -Expected @("status", "mismatch_count") -ObjectName "Restore reconciliation evidence"
            if ($document.RootElement.GetProperty("normal_project_unchanged").ValueKind -ne [Text.Json.JsonValueKind]::True -or
                $document.RootElement.GetProperty("local_rto_seconds").ValueKind -ne [Text.Json.JsonValueKind]::Number -or
                $document.RootElement.GetProperty("reconciliation").GetProperty("mismatch_count").ValueKind -ne [Text.Json.JsonValueKind]::Number) {
                throw "Restore evidence contains invalid JSON value types."
            }
            $completedAtText = $document.RootElement.GetProperty("completed_at_utc").GetString()
        }
        finally {
            $document.Dispose()
        }
        $evidence = $json | ConvertFrom-Json
    }
    catch {
        throw "Restore evidence is not valid JSON."
    }
    $backupID = Split-Path -Leaf $Backup.Directory
    if ($evidence.format_version -cne $script:LedgerSyncRestoreEvidenceFormatVersion -or
        $evidence.backup_id -cne $backupID -or $evidence.status -cne "passed" -or
        $evidence.normal_project_unchanged -cne $true) {
        throw "Restore evidence does not match the supported passed-result schema."
    }
    $completedAt = [DateTimeOffset]::MinValue
    if (-not [DateTimeOffset]::TryParse(
            $completedAtText,
            [Globalization.CultureInfo]::InvariantCulture,
            [Globalization.DateTimeStyles]::RoundtripKind,
            [ref]$completedAt
        ) -or $completedAt.Offset -ne [TimeSpan]::Zero) {
        throw "Restore evidence completion time is missing or not UTC."
    }
    if ([string]$evidence.reconciliation.status -notin @("matched", "completed", "passed") -or
        [int64]$evidence.reconciliation.mismatch_count -ne 0 -or
        [double]$evidence.local_rto_seconds -lt 0 -or [double]$evidence.local_rto_seconds -gt 86400) {
        throw "Restore evidence has an invalid reconciliation or duration result."
    }
    return [pscustomobject]@{ Path = $path; Evidence = $evidence; CompletedAt = $completedAt }
}

function Write-LedgerSyncRestoreEvidence {
    param(
        [Parameter(Mandatory = $true)][object]$Backup,
        [Parameter(Mandatory = $true)][string]$ReconciliationStatus,
        [Parameter(Mandatory = $true)][double]$LocalRTOSeconds
    )

    if ($ReconciliationStatus -notin @("matched", "completed", "passed") -or
        $LocalRTOSeconds -lt 0 -or $LocalRTOSeconds -gt 86400) {
        throw "Refusing to persist an invalid restore result."
    }
    $path = Join-Path $Backup.Directory $script:LedgerSyncRestoreEvidenceFileName
    if (Test-Path -LiteralPath $path) {
        Assert-LedgerSyncNoReparsePoints -Path $path
    }
    $pending = Join-Path $Backup.Directory ".restore-evidence.pending-$([Guid]::NewGuid().ToString('N'))"
    $payload = [ordered]@{
        format_version = $script:LedgerSyncRestoreEvidenceFormatVersion
        backup_id = Split-Path -Leaf $Backup.Directory
        completed_at_utc = [DateTimeOffset]::UtcNow.UtcDateTime.ToString("o")
        status = "passed"
        reconciliation = [ordered]@{ status = $ReconciliationStatus; mismatch_count = 0 }
        normal_project_unchanged = $true
        local_rto_seconds = [Math]::Round($LocalRTOSeconds, 2)
    }
    try {
        [IO.File]::WriteAllText($pending, ($payload | ConvertTo-Json -Depth 4), [Text.UTF8Encoding]::new($false))
        Protect-LedgerSyncRecoveryFile -Path $pending
        [IO.File]::Move($pending, $path, $true)
        Protect-LedgerSyncRecoveryFile -Path $path
        return Read-LedgerSyncRestoreEvidence -Backup $Backup
    }
    finally {
        if (Test-Path -LiteralPath $pending) { Remove-Item -LiteralPath $pending -Force }
    }
}

function New-LedgerSyncRecoveryEvidenceIndexPayload {
    param(
        [Parameter(Mandatory = $true)][string]$BackupRoot,
        [ValidateRange(1, 100)][int]$RetentionCount
    )

    $set = Get-LedgerSyncValidatedBackupSet -BackupRoot $BackupRoot
    $latest = @($set.Bundles | Select-Object -First 1)
    $latestBackup = $null
    if ($latest.Count -eq 1) {
        $bundle = $latest[0].Bundle
        $latestBackup = [ordered]@{
            backup_id = Split-Path -Leaf $bundle.Directory
            finalized_at_utc = $bundle.CreatedAt.ToUniversalTime().UtcDateTime.ToString("o")
            size_bytes = [int64]$bundle.Manifest.database.byte_length
            schema_version = [string]$bundle.Manifest.schema.migration_version
            digest_status = "verified"
            validation_status = "passed"
            source_commit = [string]$bundle.Manifest.source_commit
        }
    }
    $restoreResults = @()
    foreach ($candidate in $set.Bundles) {
        try {
            $receipt = Read-LedgerSyncRestoreEvidence -Backup $candidate.Bundle -AllowMissing
            if ($null -ne $receipt) { $restoreResults += $receipt }
        }
        catch {
            # Invalid or incomplete restore receipts are not evidence.
        }
    }
    $latestRestoreResult = @($restoreResults | Sort-Object CompletedAt -Descending | Select-Object -First 1)
    $latestRestore = $null
    if ($latestRestoreResult.Count -eq 1) {
        $receipt = $latestRestoreResult[0]
        $latestRestore = [ordered]@{
            backup_id = [string]$receipt.Evidence.backup_id
            completed_at_utc = $receipt.CompletedAt.ToUniversalTime().UtcDateTime.ToString("o")
            status = "passed"
            reconciliation_status = [string]$receipt.Evidence.reconciliation.status
            mismatch_count = 0
            normal_project_unchanged = $true
            local_rto_seconds = [double]$receipt.Evidence.local_rto_seconds
        }
    }
    return [ordered]@{
        format_version = $script:LedgerSyncRecoveryEvidenceFormatVersion
        generated_at_utc = [DateTimeOffset]::UtcNow.UtcDateTime.ToString("o")
        latest_backup = $latestBackup
        latest_restore = $latestRestore
        retention = [ordered]@{
            valid_backup_count = @($set.Bundles).Count
            ignored_entry_count = [int]$set.IgnoredCount
            configured_keep_count = $RetentionCount
        }
    }
}

function Read-LedgerSyncRecoveryEvidenceIndex {
    param([Parameter(Mandatory = $true)][string]$BackupRoot)

    $root = Resolve-LedgerSyncBackupRoot -BackupRoot $BackupRoot
    Assert-LedgerSyncNoReparsePoints -Path $root
    $path = Join-Path $root $script:LedgerSyncRecoveryEvidenceFileName
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "Recovery evidence index is missing."
    }
    Assert-LedgerSyncNoReparsePoints -Path $path
    if ($env:OS -eq "Windows_NT" -and -not (Get-Acl -LiteralPath $path).AreAccessRulesProtected) {
        throw "Recovery evidence index is not protected for local read access."
    }
    if ((Get-Item -LiteralPath $path).Length -gt 65536) {
        throw "Recovery evidence index exceeds its bounded schema size."
    }
    try {
        $json = Get-Content -LiteralPath $path -Raw
        $document = [Text.Json.JsonDocument]::Parse($json)
        try {
            $rootElement = $document.RootElement
            Assert-LedgerSyncJsonObjectProperties -Element $rootElement `
                -Expected @("format_version", "generated_at_utc", "latest_backup", "latest_restore", "retention") `
                -ObjectName "Recovery evidence index"
            Assert-LedgerSyncJsonObjectProperties -Element $rootElement.GetProperty("retention") `
                -Expected @("valid_backup_count", "ignored_entry_count", "configured_keep_count") `
                -ObjectName "Recovery retention summary"
            $retentionElement = $rootElement.GetProperty("retention")
            foreach ($property in @("valid_backup_count", "ignored_entry_count", "configured_keep_count")) {
                if ($retentionElement.GetProperty($property).ValueKind -ne [Text.Json.JsonValueKind]::Number) {
                    throw "Recovery retention fields must be JSON numbers."
                }
            }
            $generatedAtText = $rootElement.GetProperty("generated_at_utc").GetString()
            $latestBackupElement = $rootElement.GetProperty("latest_backup")
            if ($latestBackupElement.ValueKind -ne [Text.Json.JsonValueKind]::Null) {
                Assert-LedgerSyncJsonObjectProperties -Element $latestBackupElement `
                    -Expected @("backup_id", "finalized_at_utc", "size_bytes", "schema_version", "digest_status", "validation_status", "source_commit") `
                    -ObjectName "Latest backup summary"
                if ($latestBackupElement.GetProperty("size_bytes").ValueKind -ne [Text.Json.JsonValueKind]::Number) {
                    throw "Latest backup size must be a JSON number."
                }
                $latestBackupFinalizedText = $latestBackupElement.GetProperty("finalized_at_utc").GetString()
            }
            $latestRestoreElement = $rootElement.GetProperty("latest_restore")
            if ($latestRestoreElement.ValueKind -ne [Text.Json.JsonValueKind]::Null) {
                Assert-LedgerSyncJsonObjectProperties -Element $latestRestoreElement `
                    -Expected @("backup_id", "completed_at_utc", "status", "reconciliation_status", "mismatch_count", "normal_project_unchanged", "local_rto_seconds") `
                    -ObjectName "Latest restore summary"
                if ($latestRestoreElement.GetProperty("mismatch_count").ValueKind -ne [Text.Json.JsonValueKind]::Number -or
                    $latestRestoreElement.GetProperty("normal_project_unchanged").ValueKind -ne [Text.Json.JsonValueKind]::True -or
                    $latestRestoreElement.GetProperty("local_rto_seconds").ValueKind -ne [Text.Json.JsonValueKind]::Number) {
                    throw "Latest restore summary contains invalid JSON value types."
                }
                $latestRestoreCompletedText = $latestRestoreElement.GetProperty("completed_at_utc").GetString()
            }
        }
        finally {
            $document.Dispose()
        }
        $payload = $json | ConvertFrom-Json
    }
    catch {
        throw "Recovery evidence index is not valid JSON."
    }
    if ($payload.format_version -cne $script:LedgerSyncRecoveryEvidenceFormatVersion -or
        $null -eq $payload.retention -or [int]$payload.retention.valid_backup_count -lt 0 -or
        [int]$payload.retention.valid_backup_count -gt $script:LedgerSyncMaximumBackupEntries -or
        [int]$payload.retention.ignored_entry_count -lt 0 -or
        [int]$payload.retention.configured_keep_count -lt 1 -or
        [int]$payload.retention.configured_keep_count -gt 100) {
        throw "Recovery evidence index does not match the supported bounded schema."
    }
    $generatedAt = [DateTimeOffset]::MinValue
    if (-not [DateTimeOffset]::TryParse(
            $generatedAtText,
            [Globalization.CultureInfo]::InvariantCulture,
            [Globalization.DateTimeStyles]::RoundtripKind,
            [ref]$generatedAt
        ) -or $generatedAt.Offset -ne [TimeSpan]::Zero) {
        throw "Recovery evidence index generation time is invalid."
    }
    if ($null -ne $payload.latest_backup) {
        $latestBackupFinalizedAt = [DateTimeOffset]::MinValue
        if ([string]$payload.latest_backup.backup_id -cnotmatch $script:LedgerSyncBackupDirectoryPattern -or
            [string]$payload.latest_backup.source_commit -cnotmatch '^[a-f0-9]{40}$' -or
            [string]$payload.latest_backup.schema_version -cnotmatch '^[0-9]{6}_[a-z0-9._-]{1,120}$' -or
            [int64]$payload.latest_backup.size_bytes -lt 1 -or
            $payload.latest_backup.digest_status -cne "verified" -or
            $payload.latest_backup.validation_status -cne "passed") {
            throw "Latest backup summary is malformed."
        }
        if (-not [DateTimeOffset]::TryParse(
                $latestBackupFinalizedText,
                [Globalization.CultureInfo]::InvariantCulture,
                [Globalization.DateTimeStyles]::RoundtripKind,
                [ref]$latestBackupFinalizedAt
            ) -or $latestBackupFinalizedAt.Offset -ne [TimeSpan]::Zero) {
            throw "Latest backup summary timestamp is invalid."
        }
        $expectedBackupID = "backup-$($latestBackupFinalizedAt.ToUniversalTime().ToString('yyyyMMddTHHmmssZ'))-$(([string]$payload.latest_backup.source_commit).Substring(0, 7))"
        if ($payload.latest_backup.backup_id -cne $expectedBackupID) {
            throw "Latest backup summary identity is not bound to its timestamp and source commit."
        }
    }
    if ($null -ne $payload.latest_restore) {
        $latestRestoreCompletedAt = [DateTimeOffset]::MinValue
        if ([string]$payload.latest_restore.backup_id -cnotmatch $script:LedgerSyncBackupDirectoryPattern -or
            $payload.latest_restore.status -cne "passed" -or
            [string]$payload.latest_restore.reconciliation_status -notin @("matched", "completed", "passed") -or
            [int64]$payload.latest_restore.mismatch_count -ne 0 -or
            $payload.latest_restore.normal_project_unchanged -cne $true -or
            [double]$payload.latest_restore.local_rto_seconds -lt 0 -or
            [double]$payload.latest_restore.local_rto_seconds -gt 86400) {
            throw "Latest restore summary is malformed."
        }
        if (-not [DateTimeOffset]::TryParse(
                $latestRestoreCompletedText,
                [Globalization.CultureInfo]::InvariantCulture,
                [Globalization.DateTimeStyles]::RoundtripKind,
                [ref]$latestRestoreCompletedAt
            ) -or $latestRestoreCompletedAt.Offset -ne [TimeSpan]::Zero) {
            throw "Latest restore summary timestamp is invalid."
        }
    }
    $serialized = $payload | ConvertTo-Json -Depth 8 -Compress
    if ($serialized -match '(?i)([a-z]:\\|/home/|/users/|database\.dump|sha256\s*":\s*"[a-f0-9]{64}|password|secret|token)') {
        throw "Recovery evidence index contains prohibited host or credential material."
    }
    return $payload
}

function Write-LedgerSyncRecoveryEvidenceIndex {
    param(
        [Parameter(Mandatory = $true)][string]$BackupRoot,
        [ValidateRange(1, 100)][int]$RetentionCount
    )

    $root = Resolve-LedgerSyncBackupRoot -BackupRoot $BackupRoot
    Assert-LedgerSyncNoReparsePoints -Path $root
    $path = Join-Path $root $script:LedgerSyncRecoveryEvidenceFileName
    if (Test-Path -LiteralPath $path) { Assert-LedgerSyncNoReparsePoints -Path $path }
    $pending = Join-Path $root ".recovery-evidence-index.pending-$([Guid]::NewGuid().ToString('N'))"
    try {
        $payload = New-LedgerSyncRecoveryEvidenceIndexPayload `
            -BackupRoot $root -RetentionCount $RetentionCount
        [IO.File]::WriteAllText($pending, ($payload | ConvertTo-Json -Depth 8), [Text.UTF8Encoding]::new($false))
        Protect-LedgerSyncRecoveryFile -Path $pending
        [IO.File]::Move($pending, $path, $true)
        Protect-LedgerSyncRecoveryFile -Path $path
        return Read-LedgerSyncRecoveryEvidenceIndex -BackupRoot $root
    }
    finally {
        if (Test-Path -LiteralPath $pending) { Remove-Item -LiteralPath $pending -Force }
    }
}

function Initialize-LedgerSyncLocalRecoveryEvidenceIndex {
    $expectedRoot = [IO.Path]::GetFullPath((Join-Path $script:LedgerSyncRepositoryRoot "data\local-backups")).TrimEnd(
        [IO.Path]::DirectorySeparatorChar,
        [IO.Path]::AltDirectorySeparatorChar
    )
    $root = Resolve-LedgerSyncBackupRoot
    if (-not $root.Equals($expectedRoot, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Local recovery evidence initialization resolved an unexpected backup root."
    }
    Assert-LedgerSyncProspectivePathNoReparsePoints -Path $root
    New-Item -ItemType Directory -Path $root -Force | Out-Null
    Assert-LedgerSyncNoReparsePoints -Path $root
    $path = Join-Path $root $script:LedgerSyncRecoveryEvidenceFileName
    if (Test-Path -LiteralPath $path) {
        # Existing state is operator evidence. Validate it and fail closed;
        # startup must never repair, replace, or downgrade malformed evidence.
        return Read-LedgerSyncRecoveryEvidenceIndex -BackupRoot $root
    }

    $pending = Join-Path $root ".recovery-evidence-index.bootstrap-$([Guid]::NewGuid().ToString('N'))"
    $payload = [ordered]@{
        format_version = $script:LedgerSyncRecoveryEvidenceFormatVersion
        generated_at_utc = [DateTimeOffset]::UtcNow.UtcDateTime.ToString("o")
        latest_backup = $null
        latest_restore = $null
        retention = [ordered]@{
            valid_backup_count = 0
            ignored_entry_count = 0
            configured_keep_count = 5
        }
    }
    try {
        [IO.File]::WriteAllText($pending, ($payload | ConvertTo-Json -Depth 8), [Text.UTF8Encoding]::new($false))
        Protect-LedgerSyncRecoveryFile -Path $pending
        try {
            [IO.File]::Move($pending, $path, $false)
        }
        catch [IO.IOException] {
            if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw }
            # A concurrent creator won. Its file must independently satisfy the
            # exact protected schema; it is never overwritten here.
        }
        return Read-LedgerSyncRecoveryEvidenceIndex -BackupRoot $root
    }
    finally {
        if (Test-Path -LiteralPath $pending) { Remove-Item -LiteralPath $pending -Force }
    }
}
