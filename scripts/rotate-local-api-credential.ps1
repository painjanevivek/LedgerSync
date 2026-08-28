[CmdletBinding()]
param(
    [ValidateRange(30, 300)]
    [int]$WaitTimeoutSeconds = 120
)

. (Join-Path $PSScriptRoot "local-runtime-common.ps1")
. (Join-Path $PSScriptRoot "local-api-credential-common.ps1")

try {
    Assert-LedgerSyncDockerAvailable
    $services = @($script:LedgerSyncPrivateAPIDependentServices)
    $activation = {
        param([string[]]$DependentServices)
        $arguments = @(
            "up", "-d", "--no-deps", "--force-recreate", "--wait", "--wait-timeout", [string]$WaitTimeoutSeconds
        ) + @($DependentServices)
        Invoke-LedgerSyncCompose -ComposeArguments $arguments
    }
    $smoke = { Invoke-LedgerSyncWebSmoke -TimeoutSeconds 15 }

    $result = Invoke-LedgerSyncPrivateAPICredentialRotation `
        -Path $script:LedgerSyncRuntimeEnvironmentFile `
        -Activate $activation `
        -AuthenticatedSmoke $smoke

    Write-Output "Local private API credential rotation passed."
    Write-Output "Dependent services: $($services -join ',')"
    Write-Output "Fingerprint: $($result.Fingerprint)"
    Write-Output "Authenticated smoke: passed"
}
catch {
    Write-Error $_.Exception.Message
    exit 1
}
