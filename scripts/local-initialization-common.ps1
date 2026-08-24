Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$script:LedgerSyncInitializationModeFormat = "ledgersync-local-initialization/v1"
$script:LedgerSyncInitializationModeFileName = "initialization-mode.json"

function Assert-LedgerSyncInitializationMode {
    param([Parameter(Mandatory = $true)][string]$Mode)
    if ($Mode -cnotin @("demo", "empty")) {
        throw "Initialization mode must be exactly 'demo' or 'empty'."
    }
    return $Mode
}

function Get-LedgerSyncInitializationModePath {
    return Join-Path $script:LedgerSyncRuntimeStateDirectory $script:LedgerSyncInitializationModeFileName
}

function Read-LedgerSyncInitializationModeMarker {
    $path = Get-LedgerSyncInitializationModePath
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        return $null
    }
    Assert-LedgerSyncNoReparsePoints -Path $path
    if ((Get-Item -LiteralPath $path).Length -gt 4096) {
        throw "Initialization mode marker exceeds its bounded schema size."
    }
    if ($env:OS -eq "Windows_NT" -and -not (Get-Acl -LiteralPath $path).AreAccessRulesProtected) {
        throw "Initialization mode marker is not protected for local access."
    }
    try {
        $json = Get-Content -LiteralPath $path -Raw
        $document = [Text.Json.JsonDocument]::Parse($json)
        try {
            Assert-LedgerSyncJsonObjectProperties -Element $document.RootElement `
                -Expected @("format_version", "compose_project", "mode") `
                -ObjectName "Initialization mode marker"
        }
        finally {
            $document.Dispose()
        }
        $marker = $json | ConvertFrom-Json
    }
    catch {
        throw "Initialization mode marker is malformed; it was not replaced."
    }
    if ($marker.format_version -cne $script:LedgerSyncInitializationModeFormat -or
        $marker.compose_project -cne $script:LedgerSyncComposeProject) {
        throw "Initialization mode marker does not belong to the exact Compose project."
    }
    Assert-LedgerSyncInitializationMode -Mode ([string]$marker.mode) | Out-Null
    return $marker
}

function Write-LedgerSyncInitializationModeMarker {
    param([Parameter(Mandatory = $true)][string]$Mode)

    $Mode = Assert-LedgerSyncInitializationMode -Mode $Mode
    Assert-LedgerSyncProspectivePathNoReparsePoints -Path $script:LedgerSyncRuntimeStateDirectory
    New-Item -ItemType Directory -Path $script:LedgerSyncRuntimeStateDirectory -Force | Out-Null
    Assert-LedgerSyncNoReparsePoints -Path $script:LedgerSyncRuntimeStateDirectory
    $path = Get-LedgerSyncInitializationModePath
    if (Test-Path -LiteralPath $path) { Assert-LedgerSyncNoReparsePoints -Path $path }
    $pending = Join-Path $script:LedgerSyncRuntimeStateDirectory ".initialization-mode.pending-$([Guid]::NewGuid().ToString('N'))"
    $payload = [ordered]@{
        format_version = $script:LedgerSyncInitializationModeFormat
        compose_project = $script:LedgerSyncComposeProject
        mode = $Mode
    }
    try {
        [IO.File]::WriteAllText($pending, ($payload | ConvertTo-Json -Depth 3), [Text.UTF8Encoding]::new($false))
        Protect-LedgerSyncRecoveryFile -Path $pending
        [IO.File]::Move($pending, $path, $true)
        Protect-LedgerSyncRecoveryFile -Path $path
        return Read-LedgerSyncInitializationModeMarker
    }
    finally {
        if (Test-Path -LiteralPath $pending) { Remove-Item -LiteralPath $pending -Force }
    }
}

function Test-LedgerSyncPostgresVolumeExists {
    $volumeName = "$($script:LedgerSyncComposeProject)_postgres-data"
    $nativePreference = $PSNativeCommandUseErrorActionPreference
    try {
        $PSNativeCommandUseErrorActionPreference = $false
        & docker volume inspect $volumeName *> $null
        return $LASTEXITCODE -eq 0
    }
    finally {
        $PSNativeCommandUseErrorActionPreference = $nativePreference
    }
}

function Resolve-LedgerSyncInitializationModeDecision {
    param(
        [AllowNull()][string]$ExistingMode,
        [AllowNull()][string]$RequestedMode,
        [Parameter(Mandatory = $true)][bool]$PostgresVolumeExists
    )

    if (-not [string]::IsNullOrWhiteSpace($ExistingMode)) {
        $ExistingMode = Assert-LedgerSyncInitializationMode -Mode $ExistingMode
    }
    if (-not [string]::IsNullOrWhiteSpace($RequestedMode)) {
        $RequestedMode = Assert-LedgerSyncInitializationMode -Mode $RequestedMode
    }
    if (-not [string]::IsNullOrWhiteSpace($ExistingMode)) {
        if ([string]::IsNullOrWhiteSpace($RequestedMode) -or $RequestedMode -ceq $ExistingMode) {
            return $ExistingMode
        }
        if ($PostgresVolumeExists) {
            throw "Initialization mode cannot change while the exact project's PostgreSQL volume exists."
        }
        return $RequestedMode
    }
    if ($PostgresVolumeExists) {
        if ($RequestedMode -ceq "empty") {
            throw "Empty initialization cannot be adopted over an existing unmarked PostgreSQL volume."
        }
        # Backward-compatible adoption of the repository's historical seeded
        # local volume. A different choice requires explicit destructive reset.
        return "demo"
    }
    if ([string]::IsNullOrWhiteSpace($RequestedMode)) { return "demo" }
    return $RequestedMode
}

function Initialize-LedgerSyncInitializationMode {
    param([AllowNull()][string]$RequestedMode)

    $marker = Read-LedgerSyncInitializationModeMarker
    $existingMode = if ($null -eq $marker) { $null } else { [string]$marker.mode }
    $volumeExists = Test-LedgerSyncPostgresVolumeExists
    $resolvedMode = Resolve-LedgerSyncInitializationModeDecision `
        -ExistingMode $existingMode -RequestedMode $RequestedMode `
        -PostgresVolumeExists $volumeExists
    if ($null -eq $marker -or [string]$marker.mode -cne $resolvedMode) {
        $marker = Write-LedgerSyncInitializationModeMarker -Mode $resolvedMode
    }
    return [string]$marker.mode
}

function Set-LedgerSyncFreshInitializationMode {
    param([Parameter(Mandatory = $true)][string]$Mode)

    $Mode = Assert-LedgerSyncInitializationMode -Mode $Mode
    if (Test-LedgerSyncPostgresVolumeExists) {
        throw "Initialization mode can be selected only after the exact PostgreSQL volume is absent."
    }
    Write-LedgerSyncInitializationModeMarker -Mode $Mode | Out-Null
    return $Mode
}
