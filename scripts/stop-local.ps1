[CmdletBinding()]
param()

. (Join-Path $PSScriptRoot "local-runtime-common.ps1")

try {
    Assert-LedgerSyncDockerAvailable
    Invoke-LedgerSyncCompose -ComposeArguments @("stop")
    Write-Host "LedgerSync stopped. PostgreSQL and Redis volumes were preserved." -ForegroundColor Green
}
catch {
    Write-Error $_
    exit 1
}
