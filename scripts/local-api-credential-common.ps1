Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$script:LedgerSyncPrivateAPICredentialName = "LEDGERSYNC_DEVELOPMENT_API_TOKEN"
$script:LedgerSyncPrivateAPIDependentServices = @("api", "web")

function Read-LedgerSyncProtectedRuntimeEnvironment {
    param([Parameter(Mandatory = $true)][string]$Path)

    $resolvedPath = [IO.Path]::GetFullPath($Path)
    if (-not (Test-LedgerSyncRuntimeEnvironmentFile -Path $resolvedPath)) {
        throw "The protected local runtime environment is missing or malformed."
    }

    $lines = @([IO.File]::ReadAllLines($resolvedPath, [Text.UTF8Encoding]::new($false)))
    $values = @{}
    foreach ($line in $lines) {
        if ($line -cnotmatch '^([A-Z0-9_]+)=([a-f0-9]{64})$') {
            throw "The protected local runtime environment contains an unsupported entry."
        }
        $name = [string]$Matches[1]
        if ($values.ContainsKey($name)) {
            throw "The protected local runtime environment contains a duplicate entry."
        }
        $values[$name] = [string]$Matches[2]
    }
    if (-not $values.ContainsKey($script:LedgerSyncPrivateAPICredentialName)) {
        throw "The local private API credential is unavailable."
    }

    return [pscustomobject]@{
        Path = $resolvedPath
        Lines = $lines
        Values = $values
    }
}

function Test-LedgerSyncLocalSecretFileProtected {
    param([Parameter(Mandatory = $true)][string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return $false
    }
    if ($env:OS -ne "Windows_NT") {
        return $true
    }

    $acl = Get-Acl -LiteralPath $Path
    if (-not $acl.AreAccessRulesProtected) {
        return $false
    }
    $currentPrincipal = [Security.Principal.WindowsIdentity]::GetCurrent().Name
    $foreignAllow = @($acl.Access | Where-Object {
        $_.AccessControlType -eq "Allow" -and
        $_.IdentityReference.Value -ne $currentPrincipal -and
        $_.IdentityReference.Value -notmatch '^(NT AUTHORITY\\SYSTEM|BUILTIN\\Administrators)$'
    })
    return $foreignAllow.Count -eq 0
}

function Get-LedgerSyncPrivateAPICredentialRecord {
    param([Parameter(Mandatory = $true)][string]$Path)

    $runtime = Read-LedgerSyncProtectedRuntimeEnvironment -Path $Path
    if (-not (Test-LedgerSyncLocalSecretFileProtected -Path $runtime.Path)) {
        throw "The local runtime secret file is not protected for credential access."
    }
    $credential = [string]$runtime.Values[$script:LedgerSyncPrivateAPICredentialName]
    $bytes = [Convert]::FromHexString($credential)
    $digest = [Security.Cryptography.SHA256]::HashData($bytes)
    $fingerprint = -join ($digest | ForEach-Object { $_.ToString("x2") })
    [Array]::Clear($bytes, 0, $bytes.Length)
    [Array]::Clear($digest, 0, $digest.Length)

    return [pscustomobject]@{
        Name = $script:LedgerSyncPrivateAPICredentialName
        Value = $credential
        Bytes = 32
        Fingerprint = "sha256:$fingerprint"
        Protected = $true
        LastWriteTimeUtc = ([IO.FileInfo]$runtime.Path).LastWriteTimeUtc.ToString("o")
    }
}

function Get-LedgerSyncPrivateAPICredentialOutput {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [switch]$Reveal
    )

    $record = Get-LedgerSyncPrivateAPICredentialRecord -Path $Path
    if ($Reveal) {
        return [string]$record.Value
    }
    return [pscustomobject]@{
        Name = $record.Name
        Bytes = $record.Bytes
        Fingerprint = $record.Fingerprint
        Protected = $record.Protected
        LastWriteTimeUtc = $record.LastWriteTimeUtc
    }
}

function New-LedgerSyncCredentialRotationPath {
    param(
        [Parameter(Mandatory = $true)][string]$RuntimePath,
        [Parameter(Mandatory = $true)][string]$Kind
    )

    $directory = [IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($RuntimePath))
    $name = [IO.Path]::GetFileName($RuntimePath)
    return Join-Path $directory ".$name.$Kind-$([Guid]::NewGuid().ToString('N'))"
}

function Remove-LedgerSyncCredentialArtifact {
    param([string]$Path)
    if (-not [string]::IsNullOrWhiteSpace($Path) -and (Test-Path -LiteralPath $Path -PathType Leaf)) {
        Remove-Item -LiteralPath $Path -Force
    }
}

function Invoke-LedgerSyncPrivateAPICredentialRotationUnlocked {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][scriptblock]$Activate,
        [Parameter(Mandatory = $true)][scriptblock]$AuthenticatedSmoke,
        [scriptblock]$CredentialFactory = { New-LedgerSyncLocalSecret }
    )

    $runtime = Read-LedgerSyncProtectedRuntimeEnvironment -Path $Path
    if (-not (Test-LedgerSyncLocalSecretFileProtected -Path $runtime.Path)) {
        throw "The local runtime secret file is not protected for credential rotation."
    }

    $oldCredential = [string]$runtime.Values[$script:LedgerSyncPrivateAPICredentialName]
    $newCredential = [string](& $CredentialFactory)
    if ($newCredential -cnotmatch '^[a-f0-9]{64}$' -or $newCredential -ceq $oldCredential) {
        throw "Credential generation did not produce a distinct 32-byte value."
    }

    $replacementCount = 0
    $newLines = @($runtime.Lines | ForEach-Object {
        if ($_ -cmatch "^$($script:LedgerSyncPrivateAPICredentialName)=") {
            $replacementCount++
            return "$($script:LedgerSyncPrivateAPICredentialName)=$newCredential"
        }
        return $_
    })
    if ($replacementCount -ne 1) {
        throw "The local private API credential entry is ambiguous."
    }

    $pendingPath = New-LedgerSyncCredentialRotationPath -RuntimePath $runtime.Path -Kind "pending"
    $backupPath = New-LedgerSyncCredentialRotationPath -RuntimePath $runtime.Path -Kind "rollback"
    $failedPath = New-LedgerSyncCredentialRotationPath -RuntimePath $runtime.Path -Kind "failed"
    $activated = $false
    $preserveBackup = $false
    try {
        [IO.File]::WriteAllLines($pendingPath, $newLines, [Text.UTF8Encoding]::new($false))
        Protect-LedgerSyncLocalSecretFile -Path $pendingPath
        [IO.File]::Replace($pendingPath, $runtime.Path, $backupPath, $true)
        $activated = $true
        Protect-LedgerSyncLocalSecretFile -Path $runtime.Path
        Protect-LedgerSyncLocalSecretFile -Path $backupPath

        & $Activate @($script:LedgerSyncPrivateAPIDependentServices)
        & $AuthenticatedSmoke

        Remove-LedgerSyncCredentialArtifact -Path $backupPath
        return Get-LedgerSyncPrivateAPICredentialOutput -Path $runtime.Path
    }
    catch {
        if (-not $activated) {
            throw "Private API credential rotation failed before activation; protected runtime state was unchanged."
        }

        try {
            if (-not (Test-Path -LiteralPath $backupPath -PathType Leaf)) {
                throw "rollback state missing"
            }
            [IO.File]::Replace($backupPath, $runtime.Path, $failedPath, $true)
            Protect-LedgerSyncLocalSecretFile -Path $runtime.Path
            Protect-LedgerSyncLocalSecretFile -Path $failedPath
            & $Activate @($script:LedgerSyncPrivateAPIDependentServices)
            & $AuthenticatedSmoke
            Remove-LedgerSyncCredentialArtifact -Path $failedPath
        }
        catch {
            $preserveBackup = Test-Path -LiteralPath $backupPath -PathType Leaf
            throw "Private API credential activation failed and rollback could not be verified. Protected runtime state requires operator review."
        }
        throw "Private API credential activation failed; the previous credential was restored and authenticated smoke passed."
    }
    finally {
        Remove-LedgerSyncCredentialArtifact -Path $pendingPath
        if (-not $preserveBackup -and (Test-Path -LiteralPath $backupPath -PathType Leaf)) {
            Remove-LedgerSyncCredentialArtifact -Path $backupPath
        }
        if (Test-Path -LiteralPath $failedPath -PathType Leaf) {
            Remove-LedgerSyncCredentialArtifact -Path $failedPath
        }
    }
}

function Invoke-LedgerSyncPrivateAPICredentialRotation {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][scriptblock]$Activate,
        [Parameter(Mandatory = $true)][scriptblock]$AuthenticatedSmoke,
        [scriptblock]$CredentialFactory = { New-LedgerSyncLocalSecret }
    )

    $resolvedPath = [IO.Path]::GetFullPath($Path)
    $lockPath = Join-Path ([IO.Path]::GetDirectoryName($resolvedPath)) ".runtime.env.private-api-credential-rotation.lock"
    $lockStream = $null
    try {
        try {
            $lockStream = [IO.File]::Open($lockPath, [IO.FileMode]::OpenOrCreate, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
            Protect-LedgerSyncLocalSecretFile -Path $lockPath
        }
        catch {
            throw "Another private API credential rotation is already active."
        }

        return Invoke-LedgerSyncPrivateAPICredentialRotationUnlocked `
            -Path $resolvedPath `
            -Activate $Activate `
            -AuthenticatedSmoke $AuthenticatedSmoke `
            -CredentialFactory $CredentialFactory
    }
    finally {
        if ($null -ne $lockStream) {
            $lockStream.Dispose()
            Remove-LedgerSyncCredentialArtifact -Path $lockPath
        }
    }
}
