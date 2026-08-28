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

    $backupRoot = Resolve-LedgerSyncBackupRoot
    if (Test-Path -LiteralPath $backupRoot -PathType Container) {
        $backupSet = Get-LedgerSyncValidatedBackupSet -BackupRoot $backupRoot
        $latest = @($backupSet.Bundles | Select-Object -First 1)
        if ($latest.Count -eq 1) {
            $backupID = Split-Path -Leaf $latest[0].Bundle.Directory
            $restore = Read-LedgerSyncRestoreEvidence -Backup $latest[0].Bundle -AllowMissing
            $restoreStatus = if ($null -eq $restore) { "not restore-drill verified" } else { "restore-drill passed" }
            Write-Warning "Latest validated backup: $backupID ($restoreStatus). Backups are outside the Compose volumes and will be preserved."
        } else {
            Write-Warning "No validated LedgerSync backup exists. Reset will permanently remove the only known local ledger copy."
        }
    } else {
        Write-Warning "No LedgerSync backup directory exists. Reset will permanently remove the only known local ledger copy."
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
