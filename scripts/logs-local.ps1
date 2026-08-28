[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("postgres", "redis", "api", "outbox-worker", "web", "migrate", "demo-seed")]
    [string]$Service,

    [ValidateRange(1, 1000)]
    [int]$Tail = 200,

    [ValidatePattern('^[1-9][0-9]*(s|m|h)$')]
    [string]$Since = "30m"
)

. (Join-Path $PSScriptRoot "local-runtime-common.ps1")

try {
    Assert-LedgerSyncDockerAvailable
    $lines = @(Invoke-LedgerSyncCompose -ComposeArguments @("logs", "--no-color", "--timestamps", "--tail", [string]$Tail, "--since", $Since, $Service) -CaptureOutput)
    foreach ($line in $lines) {
        Write-Output (ConvertTo-LedgerSyncRedactedLogLine -Line ([string]$line))
    }
}
catch {
    Write-Error $_
    exit 1
}
