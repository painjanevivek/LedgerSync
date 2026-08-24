[CmdletBinding()]
param()

. (Join-Path $PSScriptRoot "local-runtime-common.ps1")

function Assert-Phase6 {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}

try {
    Assert-LedgerSyncDockerAvailable
    Assert-Phase6 (Test-LedgerSyncRuntimeEnvironmentFile -Path $script:LedgerSyncRuntimeEnvironmentFile) "Generated local secret state is missing or malformed."
    & git check-ignore -q -- $script:LedgerSyncRuntimeEnvironmentFile
    Assert-Phase6 ($LASTEXITCODE -eq 0) "Local runtime secrets are not ignored by Git."

    if ($env:OS -eq "Windows_NT") {
        $acl = Get-Acl -LiteralPath $script:LedgerSyncRuntimeEnvironmentFile
        Assert-Phase6 $acl.AreAccessRulesProtected "Local runtime secret ACL still inherits access rules."
        $currentPrincipal = [Security.Principal.WindowsIdentity]::GetCurrent().Name
        $foreignAllow = @($acl.Access | Where-Object {
            $_.AccessControlType -eq "Allow" -and
            $_.IdentityReference.Value -ne $currentPrincipal -and
            $_.IdentityReference.Value -notmatch '^(NT AUTHORITY\\SYSTEM|BUILTIN\\Administrators)$'
        })
        Assert-Phase6 ($foreignAllow.Count -eq 0) "Local runtime secret ACL grants another principal access."
    }

    Invoke-LedgerSyncCompose -ComposeArguments @("config", "-q")
    $containerIDs = @(Invoke-LedgerSyncCompose -ComposeArguments @("ps", "--all", "--quiet") -CaptureOutput)
    $inspected = 0
    foreach ($containerID in $containerIDs) {
        if ([string]::IsNullOrWhiteSpace([string]$containerID)) { continue }
        $container = (& docker inspect ([string]$containerID) | ConvertFrom-Json)[0]
        $service = [string]$container.Config.Labels.'com.docker.compose.service'
        Assert-Phase6 (-not [string]::IsNullOrWhiteSpace([string]$container.Config.User)) "$service runs with an implicit root user."
        Assert-Phase6 ([bool]$container.HostConfig.ReadonlyRootfs) "$service root filesystem is writable."
        Assert-Phase6 (@($container.HostConfig.CapDrop) -contains "ALL") "$service does not drop all Linux capabilities."
        Assert-Phase6 (@($container.HostConfig.SecurityOpt) -contains "no-new-privileges:true") "$service permits privilege escalation."
        $bindings = $container.HostConfig.PortBindings
        if ($service -eq "web") {
            $webBindings = @($bindings.'3000/tcp')
            Assert-Phase6 ($webBindings.Count -eq 1 -and $webBindings[0].HostIp -eq "127.0.0.1" -and $webBindings[0].HostPort -eq "3000") "Web is not bound exclusively to 127.0.0.1:3000."
        } else {
            Assert-Phase6 ($null -eq $bindings -or @($bindings.PSObject.Properties).Count -eq 0) "$service unexpectedly publishes a host port."
        }
        $inspected++
    }
    Assert-Phase6 ($inspected -ge 7) "The effective Compose inspection did not include all runtime and setup services."

    $composeSource = Get-Content -Raw -LiteralPath $script:LedgerSyncComposeFile
    Assert-Phase6 ($composeSource -notmatch 'development-only-change-me|development-local-only|:-development-') "Tracked Compose contains a fixed credential fallback."
    Assert-Phase6 ($composeSource -notmatch '(?m)^\s*image:\s*[^\r\n@]+$') "A Compose image is not digest pinned."

    $sentinel = "sentinel-private-value"
    $rawLine = "authorization: Bearer $sentinel csrf_token=$sentinel consistency_token=$sentinel database_url=postgres://ledger:$sentinel@postgres:5432/ledger balance_minor=$sentinel email=$sentinel"
    $redacted = ConvertTo-LedgerSyncRedactedLogLine -Line $rawLine
    Assert-Phase6 (-not $redacted.Contains($sentinel, [StringComparison]::Ordinal)) "Bounded local log output retained a sensitive value."

    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy
    Invoke-LedgerSyncWebSmoke
    Write-Output "Phase 6 security boundary passed: generated secrets, seven hardened containers, loopback-only publication, redacted logs, and authenticated smoke reads."
}
catch {
    Write-Error $_
    exit 1
}
