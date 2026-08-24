[CmdletBinding()]
param()

. (Join-Path $PSScriptRoot "local-runtime-common.ps1")

try {
    Assert-LedgerSyncDockerAvailable
    $rows = @(Get-LedgerSyncComposeRows)
    if ($rows.Count -eq 0) {
        Write-Host "LedgerSync is not currently created for project '$script:LedgerSyncComposeProject'."
        exit 1
    }

    $display = foreach ($service in @($script:LedgerSyncLongRunningServices + $script:LedgerSyncOneShotServices)) {
        $row = @(Get-LedgerSyncServiceRow -Service $service -Rows $rows)
        if ($row.Count -eq 0) {
            [pscustomobject]@{ Service = $service; State = "missing"; Health = "-"; SetupExit = "-" }
            continue
        }
        $health = if ($row[0].Health) { $row[0].Health } else { "-" }
        $setupExit = if ($service -in $script:LedgerSyncOneShotServices) { [string]$row[0].ExitCode } else { "-" }
        [pscustomobject]@{ Service = $service; State = $row[0].State; Health = $health; SetupExit = $setupExit }
    }

    $display | Format-Table -AutoSize

    try {
        Invoke-LedgerSyncWebSmoke -TimeoutSeconds 5
        Write-Host "Browser/BFF check: healthy ($script:LedgerSyncWebUrl)" -ForegroundColor Green
    }
    catch {
        Write-Host "Browser/BFF check: unavailable. No financial state was changed." -ForegroundColor Yellow
    }

    try {
        $summary = Get-LedgerSyncOperationalSummary
        $outboxColor = if ([int64]$summary.outbox_dead -eq 0) { "Green" } else { "Yellow" }
        Write-Host "Outbox delivery: pending=$($summary.outbox_pending), dead=$($summary.outbox_dead)" -ForegroundColor $outboxColor
        $healthyReconciliationStatuses = @("completed", "matched", "passed")
        $reconciliationColor = if ([int64]$summary.reconciliation_mismatches -eq 0 -and $summary.reconciliation_status -in $healthyReconciliationStatuses) { "Green" } else { "Yellow" }
        Write-Host "Latest reconciliation: status=$($summary.reconciliation_status), mismatches=$($summary.reconciliation_mismatches)" -ForegroundColor $reconciliationColor
    }
    catch {
        Write-Host "Operational summary: unavailable. No financial state was changed." -ForegroundColor Yellow
    }
}
catch {
    Write-Error $_
    exit 1
}
