Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$script:LedgerSyncBackupFormatVersion = "ledgersync-local-backup/v1"
$script:LedgerSyncBackupDirectoryPattern = '^backup-\d{8}T\d{6}Z-[0-9a-f]{7,40}$'

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

function Assert-LedgerSyncBackupBundle {
    param([Parameter(Mandatory = $true)][string]$BackupDirectory)

    $resolvedDirectory = [IO.Path]::GetFullPath($BackupDirectory)
    if (-not (Test-Path -LiteralPath $resolvedDirectory -PathType Container)) {
        throw "Backup directory does not exist: $resolvedDirectory"
    }

    $manifestPath = Join-Path $resolvedDirectory "manifest.json"
    if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
        throw "Backup manifest is missing: $manifestPath"
    }

    try {
        $manifestJson = Get-Content -LiteralPath $manifestPath -Raw
        $manifest = $manifestJson | ConvertFrom-Json
        $jsonDocument = [Text.Json.JsonDocument]::Parse($manifestJson)
        try {
            $createdAtText = $jsonDocument.RootElement.GetProperty("created_at_utc").GetString()
        }
        finally {
            $jsonDocument.Dispose()
        }
    }
    catch {
        throw "Backup manifest is not valid JSON: $manifestPath"
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
    if ($manifest.scope.deployment -cne "local-only" -or [string]::IsNullOrWhiteSpace([string]$manifest.scope.manifest_data)) {
        throw "Backup manifest scope is missing or is not the supported local-only boundary."
    }
    if ($manifest.database.file_name -cne "database.dump") {
        throw "Backup manifest references an unexpected database file."
    }

    $dumpPath = Join-Path $resolvedDirectory ([string]$manifest.database.file_name)
    if (-not (Test-Path -LiteralPath $dumpPath -PathType Leaf)) {
        throw "Backup database dump is missing: $dumpPath"
    }

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
    if ([string]::IsNullOrWhiteSpace([string]$manifest.schema.migration_version)) {
        throw "Backup manifest has no migration version."
    }
    if ([int64]$manifest.schema.migration_count -lt 1) {
        throw "Backup manifest has an invalid migration count."
    }

    return [pscustomobject]@{
        Directory = $resolvedDirectory
        DumpPath = $dumpPath
        ManifestPath = $manifestPath
        Manifest = $manifest
    }
}

function Test-LedgerSyncBackupCorruptionGuard {
    param([Parameter(Mandatory = $true)][string]$BackupDirectory)

    $source = Assert-LedgerSyncBackupBundle -BackupDirectory $BackupDirectory
    $suffix = [Guid]::NewGuid().ToString("N")
    $temporaryParent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    $temporaryDirectory = Join-Path $temporaryParent "ledgersync-corruption-$suffix"
    try {
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
            Assert-LedgerSyncBackupBundle -BackupDirectory $temporaryDirectory | Out-Null
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
        if (Test-Path -LiteralPath $temporaryDirectory) {
            Remove-LedgerSyncValidatedDirectory `
                -Parent $temporaryParent `
                -Directory $temporaryDirectory `
                -AllowedLeafPattern '^ledgersync-corruption-[0-9a-f]{32}$'
        }
    }
}
