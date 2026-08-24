[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repositoryRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\.."))
. (Join-Path $repositoryRoot "scripts\local-runtime-common.ps1")
. (Join-Path $repositoryRoot "scripts\local-api-credential-common.ps1")

function Assert-CredentialTooling {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}

function New-IsolatedRuntimeEnvironment {
    param([Parameter(Mandatory = $true)][string]$Path)
    $values = [ordered]@{
        POSTGRES_PASSWORD = ("01" * 32)
        LEDGERSYNC_SESSION_SECRET = ("02" * 32)
        LEDGERSYNC_CONSISTENCY_SIGNING_KEY = ("03" * 32)
        LEDGERSYNC_BFF_ASSERTION_SECRET = ("04" * 32)
        LEDGERSYNC_WEB_SESSION_SECRET = ("05" * 32)
        LEDGERSYNC_DEVELOPMENT_API_TOKEN = ("06" * 32)
    }
    $lines = @($values.GetEnumerator() | ForEach-Object { "$($_.Key)=$($_.Value)" })
    [IO.File]::WriteAllLines($Path, $lines, [Text.UTF8Encoding]::new($false))
    Protect-LedgerSyncLocalSecretFile -Path $Path
    return $values
}

$acceptanceRoot = [IO.Path]::GetFullPath((Join-Path $repositoryRoot "data\local-acceptance"))
$projectName = "ledgersync-acceptance-$([DateTime]::UtcNow.ToString('yyyyMMddHHmmss'))-$([Guid]::NewGuid().ToString('N').Substring(0, 8))"
$stateDirectory = [IO.Path]::GetFullPath((Join-Path $acceptanceRoot $projectName))
if (-not $stateDirectory.StartsWith($acceptanceRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Credential tooling test state escaped the isolated acceptance root."
}

try {
    New-Item -ItemType Directory -Force -Path $stateDirectory | Out-Null
    $runtimePath = Join-Path $stateDirectory "runtime.env"
    $initial = New-IsolatedRuntimeEnvironment -Path $runtimePath
    $privateCredential = [string]$initial.LEDGERSYNC_DEVELOPMENT_API_TOKEN
    $otherCredentials = @($initial.GetEnumerator() | Where-Object { $_.Key -ne "LEDGERSYNC_DEVELOPMENT_API_TOKEN" } | ForEach-Object { [string]$_.Value })

    Assert-CredentialTooling (Test-LedgerSyncLocalSecretFileProtected -Path $runtimePath) "Isolated runtime credential file is not protected."

    $metadata = Get-LedgerSyncPrivateAPICredentialOutput -Path $runtimePath
    $metadataText = $metadata | Out-String
    Assert-CredentialTooling ($metadata.Fingerprint -match '^sha256:[a-f0-9]{64}$') "Default credential output omitted its SHA-256 fingerprint."
    Assert-CredentialTooling ($metadata.Bytes -eq 32 -and $metadata.Protected) "Default credential metadata is incomplete."
    Assert-CredentialTooling (-not $metadataText.Contains($privateCredential, [StringComparison]::Ordinal)) "Default credential output revealed the private API credential."
    foreach ($credential in $otherCredentials) {
        Assert-CredentialTooling (-not $metadataText.Contains($credential, [StringComparison]::Ordinal)) "Default credential output revealed another runtime credential."
    }

    $revealed = [string](Get-LedgerSyncPrivateAPICredentialOutput -Path $runtimePath -Reveal)
    Assert-CredentialTooling ($revealed -ceq $privateCredential) "Explicit reveal did not return exactly the private API credential."
    foreach ($credential in $otherCredentials) {
        Assert-CredentialTooling (-not $revealed.Contains($credential, [StringComparison]::Ordinal)) "Explicit reveal returned another runtime credential."
    }

    $lockPath = Join-Path $stateDirectory ".runtime.env.private-api-credential-rotation.lock"
    $heldLock = [IO.File]::Open($lockPath, [IO.FileMode]::OpenOrCreate, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
    try {
        $concurrentRejected = $false
        try {
            Invoke-LedgerSyncPrivateAPICredentialRotation `
                -Path $runtimePath `
                -CredentialFactory { "09" * 32 } `
                -Activate { throw "concurrent activation must not run" } `
                -AuthenticatedSmoke { throw "concurrent smoke must not run" } | Out-Null
        }
        catch {
            $concurrentRejected = $_.Exception.Message -eq "Another private API credential rotation is already active."
        }
        Assert-CredentialTooling $concurrentRejected "A concurrent credential rotation was not rejected before activation."
        $afterConcurrentAttempt = Read-LedgerSyncProtectedRuntimeEnvironment -Path $runtimePath
        Assert-CredentialTooling ([string]$afterConcurrentAttempt.Values.LEDGERSYNC_DEVELOPMENT_API_TOKEN -ceq $privateCredential) "A rejected concurrent rotation changed protected state."
    }
    finally {
        $heldLock.Dispose()
        Remove-LedgerSyncCredentialArtifact -Path $lockPath
    }

    $script:activationCalls = @()
    $script:smokeCalls = 0
    $newCredential = "07" * 32
    $rotationOutput = @(
        Invoke-LedgerSyncPrivateAPICredentialRotation `
            -Path $runtimePath `
            -CredentialFactory { $newCredential } `
            -Activate {
                param([string[]]$DependentServices)
                $script:activationCalls += ,@($DependentServices)
            } `
            -AuthenticatedSmoke { $script:smokeCalls++ }
    )
    $rotated = Read-LedgerSyncProtectedRuntimeEnvironment -Path $runtimePath
    Assert-CredentialTooling ([string]$rotated.Values.LEDGERSYNC_DEVELOPMENT_API_TOKEN -ceq $newCredential) "Rotation did not activate the new private API credential."
    Assert-CredentialTooling ($script:activationCalls.Count -eq 1 -and (@($script:activationCalls[0]) -join ',') -ceq 'api,web') "Rotation did not recreate exactly api and web."
    Assert-CredentialTooling ($script:smokeCalls -eq 1) "Rotation did not run one authenticated smoke verification."
    foreach ($entry in $initial.GetEnumerator()) {
        if ($entry.Key -eq "LEDGERSYNC_DEVELOPMENT_API_TOKEN") { continue }
        Assert-CredentialTooling ([string]$rotated.Values[$entry.Key] -ceq [string]$entry.Value) "Rotation changed an unrelated runtime credential."
    }
    $rotationText = $rotationOutput | Out-String
    foreach ($credential in @($privateCredential, $newCredential) + $otherCredentials) {
        Assert-CredentialTooling (-not $rotationText.Contains($credential, [StringComparison]::Ordinal)) "Rotation output disclosed runtime credential material."
    }
    Assert-CredentialTooling (Test-LedgerSyncLocalSecretFileProtected -Path $runtimePath) "Rotated runtime credential file lost its protected ACL."

    $script:activationCalls = @()
    $script:smokeCalls = 0
    $rollbackCredential = "08" * 32
    $rollbackFailed = $false
    try {
        Invoke-LedgerSyncPrivateAPICredentialRotation `
            -Path $runtimePath `
            -CredentialFactory { $rollbackCredential } `
            -Activate {
                param([string[]]$DependentServices)
                $script:activationCalls += ,@($DependentServices)
            } `
            -AuthenticatedSmoke {
                $script:smokeCalls++
                if ($script:smokeCalls -eq 1) { throw "isolated activation failure" }
            } | Out-Null
    }
    catch {
        $rollbackFailed = $true
        $errorText = $_.Exception.Message
        foreach ($credential in @($privateCredential, $newCredential, $rollbackCredential) + $otherCredentials) {
            Assert-CredentialTooling (-not $errorText.Contains($credential, [StringComparison]::Ordinal)) "Rollback error disclosed runtime credential material."
        }
    }
    Assert-CredentialTooling $rollbackFailed "Failed activation did not report rotation failure."
    $rolledBack = Read-LedgerSyncProtectedRuntimeEnvironment -Path $runtimePath
    Assert-CredentialTooling ([string]$rolledBack.Values.LEDGERSYNC_DEVELOPMENT_API_TOKEN -ceq $newCredential) "Failed activation did not restore the previous credential."
    Assert-CredentialTooling ($script:activationCalls.Count -eq 2 -and $script:smokeCalls -eq 2) "Rollback did not reactivate and smoke the previous credential."
    Assert-CredentialTooling (@(Get-ChildItem -LiteralPath $stateDirectory -File | Where-Object { $_.Name -ne "runtime.env" }).Count -eq 0) "Credential rotation left a pending or failed secret artifact."

    $apiRoutes = @(Get-ChildItem -LiteralPath (Join-Path $repositoryRoot "web\src\app\api") -Recurse -Filter "route.ts" -File)
    foreach ($route in $apiRoutes) {
        $source = Get-Content -Raw -LiteralPath $route.FullName
        Assert-CredentialTooling ($source -notmatch 'LEDGERSYNC_(?:DEVELOPMENT_API_TOKEN|PRIVATE_API_TOKEN)|runtime\.env|local-api-credential|rotate-local-api') "A browser API route references raw credential tooling or state."
        Assert-CredentialTooling ($source -notmatch 'searchParams\.get\(["''](?:url|targetUrl|headers)["'']\)') "A browser API route accepts an arbitrary request target or headers."
    }
    $forbiddenRouteNames = @($apiRoutes | Where-Object { $_.FullName -match '(?i)(credential|token|reveal|rotate|request-runner)' })
    Assert-CredentialTooling ($forbiddenRouteNames.Count -eq 0) "A browser-visible credential or arbitrary request route exists."

    Write-Output "Local private API credential tooling tests passed: protected metadata, deliberate reveal, exact-service rotation, authenticated smoke, rollback, redaction, and browser boundary."
}
finally {
    if (Test-Path -LiteralPath $stateDirectory -PathType Container) {
        $resolvedCleanup = [IO.Path]::GetFullPath($stateDirectory)
        if (-not $resolvedCleanup.StartsWith($acceptanceRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetFileName($resolvedCleanup) -cnotmatch '^ledgersync-acceptance-\d{14}-[0-9a-f]{8}$') {
            throw "Refusing to remove an unverified credential tooling test directory."
        }
        Remove-Item -LiteralPath $resolvedCleanup -Recurse -Force
    }
}
