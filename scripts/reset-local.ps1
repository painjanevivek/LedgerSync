[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Confirmation,
    [ValidateSet("demo", "empty")]
    [string]$InitializationMode
)

. (Join-Path $PSScriptRoot "local-runtime-common.ps1")
. (Join-Path $PSScriptRoot "local-backup-common.ps1")
. (Join-Path $PSScriptRoot "local-initialization-common.ps1")

$requiredConfirmation = "DELETE LEDGERSYNC LOCAL DATA"
if ($Confirmation -cne $requiredConfirmation) {
    Write-Error "Reset refused. To delete only project '$script:LedgerSyncComposeProject', pass -Confirmation '$requiredConfirmation'."
    exit 1
}

try {
    Assert-LedgerSyncDockerAvailable
    if ($script:LedgerSyncComposeProject -ne "compose" -and -not $env:LEDGERSYNC_LOCAL_COMPOSE_PROJECT) {
        throw "Reset refused because the exact Compose project was not explicitly resolved."
    }

    Write-Warning "Deleting LedgerSync containers and named PostgreSQL/Redis volumes for project '$script:LedgerSyncComposeProject'."
    Invoke-LedgerSyncCompose -ComposeArguments @("down", "--volumes", "--remove-orphans")
    if ($PSBoundParameters.ContainsKey("InitializationMode")) {
        Set-LedgerSyncFreshInitializationMode -Mode $InitializationMode | Out-Null
    }
    Write-Host "The exact LedgerSync local project and its data volumes were deleted. This cannot be undone from this command." -ForegroundColor Yellow
    if ($PSBoundParameters.ContainsKey("InitializationMode")) {
        Write-Host "The next fresh startup is pinned to initialization mode '$InitializationMode'." -ForegroundColor Yellow
    }
}
catch {
    Write-Error $_
    exit 1
}
