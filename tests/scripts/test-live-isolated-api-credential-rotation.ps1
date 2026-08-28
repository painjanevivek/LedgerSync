[CmdletBinding()]
param(
    [switch]$ConfirmIsolatedComposeCredentialRotation,
    [ValidateRange(30, 300)]
    [int]$WaitTimeoutSeconds = 120
)

$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

if (-not $ConfirmIsolatedComposeCredentialRotation) {
    throw "This live test is destructive only to a new isolated Compose project. Re-run with -ConfirmIsolatedComposeCredentialRotation."
}

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$scriptsRoot = Join-Path $repositoryRoot "scripts"
$composeFile = Join-Path $repositoryRoot "deploy\compose\docker-compose.yml"
$acceptanceProject = "ledgersync-acceptance-$((Get-Date).ToUniversalTime().ToString('yyyyMMddHHmmss'))-$([Guid]::NewGuid().ToString('N').Substring(0, 8))"
$acceptanceRoot = [IO.Path]::GetFullPath((Join-Path $repositoryRoot "data\local-acceptance"))
$acceptanceState = [IO.Path]::GetFullPath((Join-Path $acceptanceRoot $acceptanceProject))
$runtimePath = Join-Path $acceptanceState "runtime.env"
$previousProjectEnvironment = [Environment]::GetEnvironmentVariable("LEDGERSYNC_LOCAL_COMPOSE_PROJECT")
$previousStateEnvironment = [Environment]::GetEnvironmentVariable("LEDGERSYNC_LOCAL_STATE_DIRECTORY")
$previousComposeProgress = [Environment]::GetEnvironmentVariable("COMPOSE_PROGRESS")
$normalWasStopped = $false
$acceptanceMayExist = $false
$normalBefore = $null
$failurePhase = "preflight"
$failure = $null
$cleanupFailure = $null
$capturedOutput = [Collections.Generic.List[object]]::new()
$failureDiagnostics = [Collections.Generic.List[object]]::new()
$secretValues = [Collections.Generic.List[string]]::new()
$expectedServices = @("postgres", "redis", "migrate", "demo-seed", "api", "outbox-worker", "web")
$unrelatedServices = @("postgres", "redis", "migrate", "demo-seed", "outbox-worker")

function Assert-IsolatedProjectIdentity {
    if ($acceptanceProject -cnotmatch '^ledgersync-acceptance-\d{14}-[0-9a-f]{8}$') {
        throw "Generated acceptance project identity is invalid."
    }
    if ($acceptanceProject -ceq "compose") {
        throw "The live credential test refuses the normal Compose project."
    }
    $expectedState = [IO.Path]::GetFullPath((Join-Path $acceptanceRoot $acceptanceProject))
    if (-not $acceptanceState.Equals($expectedState, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Generated acceptance state path is invalid."
    }
}

function Get-ProjectDockerResources {
    param([Parameter(Mandatory = $true)][string]$Project)

    $containers = @(& docker ps -a --filter "label=com.docker.compose.project=$Project" --format '{{.ID}}')
    if ($LASTEXITCODE -ne 0) { throw "Could not inspect isolated Compose containers." }
    $volumes = @(& docker volume ls --filter "label=com.docker.compose.project=$Project" --format '{{.Name}}')
    if ($LASTEXITCODE -ne 0) { throw "Could not inspect isolated Compose volumes." }
    $networks = @(& docker network ls --filter "label=com.docker.compose.project=$Project" --format '{{.ID}}')
    if ($LASTEXITCODE -ne 0) { throw "Could not inspect isolated Compose networks." }
    return [pscustomobject]@{
        Containers = @($containers | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) })
        Volumes = @($volumes | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) })
        Networks = @($networks | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) })
    }
}

function Get-ServiceContainerIDs {
    $result = @{}
    foreach ($service in $expectedServices) {
        $ids = @(& docker ps -a `
            --filter "label=com.docker.compose.project=$acceptanceProject" `
            --filter "label=com.docker.compose.service=$service" `
            --format '{{.ID}}')
        if ($LASTEXITCODE -ne 0 -or $ids.Count -ne 1 -or [string]::IsNullOrWhiteSpace([string]$ids[0])) {
            throw "Acceptance service ownership is incomplete for '$service'."
        }
        $container = @(& docker inspect ([string]$ids[0]) | ConvertFrom-Json)
        if ($LASTEXITCODE -ne 0 -or $container.Count -ne 1) {
            throw "Acceptance service ownership could not be inspected for '$service'."
        }
        $labels = $container[0].Config.Labels
        if ([string]$labels.'com.docker.compose.project' -cne $acceptanceProject -or
            [string]$labels.'com.docker.compose.service' -cne $service) {
            throw "Acceptance service ownership labels are invalid for '$service'."
        }
        $result[$service] = [string]$ids[0]
    }
    return $result
}

function Assert-OnlyDependentServicesRecreated {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Before,
        [Parameter(Mandatory = $true)][hashtable]$After
    )

    foreach ($service in @("api", "web")) {
        if ([string]$Before[$service] -ceq [string]$After[$service]) {
            throw "Credential activation did not recreate '$service'."
        }
    }
    foreach ($service in $unrelatedServices) {
        if ([string]$Before[$service] -cne [string]$After[$service]) {
            throw "Credential activation unexpectedly recreated '$service'."
        }
    }
}

function Get-SecretFingerprints {
    param([Parameter(Mandatory = $true)][string]$Path)

    $runtime = Read-LedgerSyncProtectedRuntimeEnvironment -Path $Path
    $fingerprints = @{}
    foreach ($name in $runtime.Values.Keys) {
        $value = [string]$runtime.Values[$name]
        if (-not $secretValues.Contains($value)) { $secretValues.Add($value) }
        $bytes = [Convert]::FromHexString($value)
        $digest = [Security.Cryptography.SHA256]::HashData($bytes)
        $fingerprints[[string]$name] = -join ($digest | ForEach-Object { $_.ToString("x2") })
        [Array]::Clear($bytes, 0, $bytes.Length)
        [Array]::Clear($digest, 0, $digest.Length)
    }
    return $fingerprints
}

function Assert-UnrelatedSecretFingerprintsUnchanged {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Before,
        [Parameter(Mandatory = $true)][hashtable]$After
    )

    foreach ($name in $Before.Keys) {
        if ([string]$name -ceq "LEDGERSYNC_DEVELOPMENT_API_TOKEN") { continue }
        if (-not $After.ContainsKey($name) -or [string]$Before[$name] -cne [string]$After[$name]) {
            throw "An unrelated protected runtime credential changed during private API credential rotation."
        }
    }
}

function Assert-NoSecretInCapturedOutput {
    $joined = ($capturedOutput | ForEach-Object { [string]$_ }) -join "`n"
    foreach ($value in $secretValues) {
        if (-not [string]::IsNullOrWhiteSpace($value) -and $joined.Contains($value, [StringComparison]::Ordinal)) {
            throw "A raw protected runtime credential appeared in captured command output."
        }
    }
}

function Invoke-CapturedPowerShellScript {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [string[]]$Arguments = @()
    )

    $nativePreference = $PSNativeCommandUseErrorActionPreference
    try {
        $PSNativeCommandUseErrorActionPreference = $false
        $output = @(& pwsh -NoProfile -File $Path @Arguments *>&1)
        $exitCode = $LASTEXITCODE
    }
    finally {
        $PSNativeCommandUseErrorActionPreference = $nativePreference
    }
    return [pscustomobject]@{ Output = $output; ExitCode = $exitCode }
}

function Write-RedactedCapturedDiagnostics {
    $lines = @($failureDiagnostics | Select-Object -Last 80)
    foreach ($line in $lines) {
        $raw = [string]$line
        if ([string]::IsNullOrWhiteSpace($raw)) { continue }
        $safe = ConvertTo-LedgerSyncRedactedLogLine -Line $raw
        $safe = $safe -replace '(?<![a-fA-F0-9])[a-fA-F0-9]{64}(?![a-fA-F0-9])', '[REDACTED-64-HEX]'
        if (-not [string]::IsNullOrWhiteSpace($safe)) {
            Write-Warning $safe
        }
    }
}

function Remove-AcceptanceStateSafely {
    if (-not (Test-Path -LiteralPath $acceptanceState)) { return }
    Assert-IsolatedProjectIdentity
    $item = Get-Item -LiteralPath $acceptanceState -Force
    if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Acceptance cleanup refused a reparse-point state path."
    }
    Remove-Item -LiteralPath $acceptanceState -Recurse -Force
}

try {
    Assert-IsolatedProjectIdentity
    if (-not [string]::IsNullOrWhiteSpace($previousProjectEnvironment) -or
        -not [string]::IsNullOrWhiteSpace($previousStateEnvironment)) {
        throw "The live test requires unset LedgerSync project/state overrides so it can prove normal and isolated ownership."
    }

    . (Join-Path $scriptsRoot "local-runtime-common.ps1")
    . (Join-Path $scriptsRoot "local-api-credential-common.ps1")
    if ($script:LedgerSyncComposeProject -cne "compose") {
        throw "The normal Compose preflight did not resolve the exact 'compose' project."
    }
    Assert-LedgerSyncDockerAvailable
    $collision = Get-ProjectDockerResources -Project $acceptanceProject
    if ($collision.Containers.Count -ne 0 -or $collision.Volumes.Count -ne 0 -or
        $collision.Networks.Count -ne 0 -or (Test-Path -LiteralPath $acceptanceState)) {
        throw "The generated acceptance project or state path already exists."
    }

    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy
    Invoke-LedgerSyncWebSmoke
    $normalBefore = Get-LedgerSyncFinancialFingerprint

    $failurePhase = "normal project stop"
    $stopResult = Invoke-CapturedPowerShellScript -Path (Join-Path $scriptsRoot "stop-local.ps1")
    foreach ($item in $stopResult.Output) { $capturedOutput.Add($item) }
    if ($stopResult.ExitCode -ne 0) { throw "The normal project could not be stopped without deleting its data." }
    $normalWasStopped = $true

    $failurePhase = "isolated project start command"
    $env:LEDGERSYNC_LOCAL_COMPOSE_PROJECT = $acceptanceProject
    $env:LEDGERSYNC_LOCAL_STATE_DIRECTORY = $acceptanceState
    $env:COMPOSE_PROGRESS = "quiet"
    . (Join-Path $scriptsRoot "local-runtime-common.ps1")
    . (Join-Path $scriptsRoot "local-api-credential-common.ps1")
    if ($script:LedgerSyncComposeProject -cne $acceptanceProject -or
        -not $script:LedgerSyncRuntimeStateDirectory.Equals($acceptanceState, [StringComparison]::OrdinalIgnoreCase)) {
        throw "The isolated runtime helpers did not retain exact project/state ownership."
    }
    New-Item -ItemType Directory -Path $acceptanceRoot -Force | Out-Null
    $acceptanceMayExist = $true
    $startResult = Invoke-CapturedPowerShellScript -Path (Join-Path $scriptsRoot "start-local.ps1")
    foreach ($item in $startResult.Output) { $capturedOutput.Add($item) }
    if ($startResult.ExitCode -ne 0) { throw "The isolated Compose stack did not start." }
    $failurePhase = "isolated service health verification"
    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy
    Invoke-LedgerSyncWebSmoke

    $failurePhase = "isolated resource ownership verification"
    $resources = Get-ProjectDockerResources -Project $acceptanceProject
    if ($resources.Containers.Count -ne $expectedServices.Count -or $resources.Volumes.Count -lt 2 -or $resources.Networks.Count -lt 1) {
        throw "The isolated Compose project did not create its exact expected resource set."
    }
    $initialIDs = Get-ServiceContainerIDs
    $failurePhase = "isolated secret fingerprint baseline"
    $initialFingerprints = Get-SecretFingerprints -Path $runtimePath
    Assert-NoSecretInCapturedOutput

    $failurePhase = "successful credential rotation"
    $successfulCandidate = New-LedgerSyncLocalSecret
    $secretValues.Add($successfulCandidate)
    $successfulContext = @{ ActivationCount = 0 }
    $successfulActivation = {
        param([string[]]$Services)
        if ((@($Services) -join ",") -cne "api,web") {
            throw "Credential rotation requested an unexpected dependent service set."
        }
        $successfulContext.ActivationCount++
        $activationOutput = @(Invoke-LedgerSyncCompose -ComposeArguments @(
            "up", "-d", "--no-deps", "--force-recreate", "--wait", "--wait-timeout", [string]$WaitTimeoutSeconds,
            "api", "web"
        ) *>&1)
        foreach ($item in $activationOutput) { $capturedOutput.Add($item) }
    }
    $successfulSmoke = { Invoke-LedgerSyncWebSmoke }
    $rotationOutput = @(& {
        Invoke-LedgerSyncPrivateAPICredentialRotation -Path $runtimePath `
            -Activate $successfulActivation -AuthenticatedSmoke $successfulSmoke `
            -CredentialFactory { $successfulCandidate }
    } *>&1)
    foreach ($item in $rotationOutput) { $capturedOutput.Add($item) }
    if ($successfulContext.ActivationCount -ne 1) {
        throw "Successful credential rotation did not perform exactly one activation."
    }
    $successfulIDs = Get-ServiceContainerIDs
    Assert-OnlyDependentServicesRecreated -Before $initialIDs -After $successfulIDs
    $successfulFingerprints = Get-SecretFingerprints -Path $runtimePath
    Assert-UnrelatedSecretFingerprintsUnchanged -Before $initialFingerprints -After $successfulFingerprints
    if ([string]$initialFingerprints["LEDGERSYNC_DEVELOPMENT_API_TOKEN"] -ceq
        [string]$successfulFingerprints["LEDGERSYNC_DEVELOPMENT_API_TOKEN"]) {
        throw "Successful credential rotation did not change the target credential fingerprint."
    }
    Invoke-LedgerSyncWebSmoke

    $failurePhase = "credential rollback"
    $rollbackCandidate = New-LedgerSyncLocalSecret
    $secretValues.Add($rollbackCandidate)
    $rollbackContext = @{ ActivationCount = 0; SmokeCount = 0 }
    $rollbackActivationIDs = [Collections.Generic.List[hashtable]]::new()
    $rollbackActivation = {
        param([string[]]$Services)
        if ((@($Services) -join ",") -cne "api,web") {
            throw "Credential rollback requested an unexpected dependent service set."
        }
        $rollbackContext.ActivationCount++
        $activationOutput = @(Invoke-LedgerSyncCompose -ComposeArguments @(
            "up", "-d", "--no-deps", "--force-recreate", "--wait", "--wait-timeout", [string]$WaitTimeoutSeconds,
            "api", "web"
        ) *>&1)
        foreach ($item in $activationOutput) { $capturedOutput.Add($item) }
        $rollbackActivationIDs.Add((Get-ServiceContainerIDs))
    }
    $rollbackSmoke = {
        $rollbackContext.SmokeCount++
        if ($rollbackContext.SmokeCount -eq 1) {
            Invoke-LedgerSyncWebSmoke
            throw "Deliberate isolated activation-smoke failure."
        }
        Invoke-LedgerSyncWebSmoke
    }
    $expectedRollbackError = $false
    try {
        $rollbackOutput = @(& {
            Invoke-LedgerSyncPrivateAPICredentialRotation -Path $runtimePath `
                -Activate $rollbackActivation -AuthenticatedSmoke $rollbackSmoke `
                -CredentialFactory { $rollbackCandidate }
        } *>&1)
        foreach ($item in $rollbackOutput) { $capturedOutput.Add($item) }
    }
    catch {
        $capturedOutput.Add($_.Exception.Message)
        $expectedRollbackError = $_.Exception.Message -ceq
            "Private API credential activation failed; the previous credential was restored and authenticated smoke passed."
    }
    if (-not $expectedRollbackError -or $rollbackContext.ActivationCount -ne 2 -or
        $rollbackContext.SmokeCount -ne 2 -or $rollbackActivationIDs.Count -ne 2) {
        throw "The injected activation-smoke failure did not complete one verified real rollback."
    }
    Assert-OnlyDependentServicesRecreated -Before $successfulIDs -After $rollbackActivationIDs[0]
    Assert-OnlyDependentServicesRecreated -Before $rollbackActivationIDs[0] -After $rollbackActivationIDs[1]
    $rollbackFingerprints = Get-SecretFingerprints -Path $runtimePath
    Assert-UnrelatedSecretFingerprintsUnchanged -Before $initialFingerprints -After $rollbackFingerprints
    if ([string]$rollbackFingerprints["LEDGERSYNC_DEVELOPMENT_API_TOKEN"] -cne
        [string]$successfulFingerprints["LEDGERSYNC_DEVELOPMENT_API_TOKEN"]) {
        throw "Verified rollback did not restore the immediately prior private API credential fingerprint."
    }
    Invoke-LedgerSyncWebSmoke
    Assert-NoSecretInCapturedOutput

    Write-Output "ISOLATED_CREDENTIAL_ROTATION=PASS"
    Write-Output "PROJECT_OWNERSHIP=PASS"
    Write-Output "SUCCESSFUL_ROTATION=PASS"
    Write-Output "ROLLBACK_RECREATION=PASS"
    Write-Output "UNRELATED_SECRET_FINGERPRINTS=UNCHANGED"
    Write-Output "RAW_SECRET_OUTPUT=ABSENT"
}
catch {
    $failure = $_
    foreach ($item in @($capturedOutput | Select-Object -Last 80)) { $failureDiagnostics.Add($item) }
}
finally {
    $PSNativeCommandUseErrorActionPreference = $false
    try {
        Assert-IsolatedProjectIdentity
        if ($acceptanceMayExist) {
            if (Test-Path -LiteralPath $runtimePath -PathType Leaf) {
                & docker compose --env-file $runtimePath -p $acceptanceProject -f $composeFile `
                    down --volumes --remove-orphans --timeout 10 *> $null
            }

            # Compose may have failed before it could perform `down`. Resolve
            # fallback cleanup only from the exact generated project label.
            $remaining = Get-ProjectDockerResources -Project $acceptanceProject
            if ($remaining.Containers.Count -gt 0) {
                & docker rm -f @($remaining.Containers) *> $null
            }
            $remaining = Get-ProjectDockerResources -Project $acceptanceProject
            if ($remaining.Volumes.Count -gt 0) {
                & docker volume rm -f @($remaining.Volumes) *> $null
            }
            if ($remaining.Networks.Count -gt 0) {
                & docker network rm @($remaining.Networks) *> $null
            }
        }
        Remove-AcceptanceStateSafely
        $remaining = Get-ProjectDockerResources -Project $acceptanceProject
        if ($remaining.Containers.Count -ne 0 -or $remaining.Volumes.Count -ne 0 -or
            $remaining.Networks.Count -ne 0 -or (Test-Path -LiteralPath $acceptanceState)) {
            throw "Isolated acceptance cleanup left project-owned resources or state behind."
        }
        Write-Output "ISOLATED_CLEANUP=PASS"
    }
    catch {
        $cleanupFailure = $_
    }

    Remove-Item Env:LEDGERSYNC_LOCAL_COMPOSE_PROJECT -ErrorAction SilentlyContinue
    Remove-Item Env:LEDGERSYNC_LOCAL_STATE_DIRECTORY -ErrorAction SilentlyContinue
    if ($null -eq $previousComposeProgress) {
        Remove-Item Env:COMPOSE_PROGRESS -ErrorAction SilentlyContinue
    } else {
        $env:COMPOSE_PROGRESS = $previousComposeProgress
    }

    if ($normalWasStopped) {
        try {
            $restoreResult = Invoke-CapturedPowerShellScript `
                -Path (Join-Path $scriptsRoot "start-local.ps1") -Arguments @("-SkipBuild")
            foreach ($item in $restoreResult.Output) { $capturedOutput.Add($item) }
            if ($restoreResult.ExitCode -ne 0) { throw "The normal project did not restart." }
            . (Join-Path $scriptsRoot "local-runtime-common.ps1")
            Assert-LedgerSyncLongRunningServicesHealthy
            Invoke-LedgerSyncWebSmoke
            Compare-LedgerSyncFinancialFingerprint -Before $normalBefore -After (Get-LedgerSyncFinancialFingerprint)
            Write-Output "NORMAL_PROJECT_RESTORED=PASS"
        }
        catch {
            if ($null -eq $cleanupFailure) { $cleanupFailure = $_ }
        }
    }

    if ($null -ne $previousProjectEnvironment) {
        $env:LEDGERSYNC_LOCAL_COMPOSE_PROJECT = $previousProjectEnvironment
    }
    if ($null -ne $previousStateEnvironment) {
        $env:LEDGERSYNC_LOCAL_STATE_DIRECTORY = $previousStateEnvironment
    }
}

if ($null -ne $cleanupFailure) {
    throw "Live isolated credential test cleanup or normal-project restoration failed. Manual bounded inspection is required."
}
if ($null -ne $failure) {
    Write-RedactedCapturedDiagnostics
    throw "Live isolated credential test failed during $failurePhase. All generated credentials remained redacted."
}
