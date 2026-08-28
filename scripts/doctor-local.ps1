[CmdletBinding()]
param()

. (Join-Path $PSScriptRoot "local-runtime-common.ps1")

try {
    Write-Host "LedgerSync local doctor (read-only)" -ForegroundColor Cyan
    Write-Host "Project: $script:LedgerSyncComposeProject | Repository: $script:LedgerSyncRepositoryRoot"
    $checks = @(Get-LedgerSyncLocalDoctorChecks)
    $checks | Select-Object Check, Status, Detail | Format-Table -AutoSize -Wrap

    $problems = @($checks | Where-Object { $_.Status -eq "fail" })
    if ($problems.Count -gt 0) {
        Write-Host "Recovery actions:" -ForegroundColor Yellow
        foreach ($problem in $problems) {
            Write-Host "- $($problem.Check): $($problem.NextAction)"
        }
        Write-Host "No containers, volumes, data, or secret files were changed." -ForegroundColor Yellow
        exit 1
    }

    Write-Host "Local prerequisites are ready. No containers, volumes, data, or secret files were changed." -ForegroundColor Green
}
catch {
    Write-Error $_
    exit 1
}
