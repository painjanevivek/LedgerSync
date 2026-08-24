[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Confirmation
)

. (Join-Path $PSScriptRoot "local-runtime-common.ps1")

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
    Write-Host "The exact LedgerSync local project and its data volumes were deleted. This cannot be undone from this command." -ForegroundColor Yellow
}
catch {
    Write-Error $_
    exit 1
}
