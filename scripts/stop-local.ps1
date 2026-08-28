[CmdletBinding()]
param()

. (Join-Path $PSScriptRoot "local-runtime-common.ps1")

try {
    Assert-LedgerSyncDockerAvailable
    Invoke-LedgerSyncCompose -ComposeArguments @("stop", "--timeout", "30")
    Write-Host "LedgerSync stopped after a bounded graceful-shutdown window. PostgreSQL and Redis volumes were preserved." -ForegroundColor Green
}
catch {
    Write-Error $_
    exit 1
}
