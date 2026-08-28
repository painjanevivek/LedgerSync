Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Assert-LedgerSyncRetryLabIdentity {
    param(
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [Parameter(Mandatory = $true)][string]$Project,
        [Parameter(Mandatory = $true)][string]$StateDirectory
    )

    if ($Project -cnotmatch '^ledgersync-acceptance-\d{14}-[0-9a-f]{8}$' -or $Project -ceq "compose") {
        throw "Retry lab project must be a generated LedgerSync acceptance identity."
    }
    $root = [IO.Path]::GetFullPath($RepositoryRoot)
    $acceptanceRoot = [IO.Path]::GetFullPath((Join-Path $root "data\local-acceptance"))
    $expectedState = [IO.Path]::GetFullPath((Join-Path $acceptanceRoot $Project))
    $resolvedState = [IO.Path]::GetFullPath($StateDirectory)
    if (-not $resolvedState.Equals($expectedState, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Retry lab state must exactly match data/local-acceptance/<generated-project>."
    }
    if (Test-Path -LiteralPath $resolvedState) {
        $item = Get-Item -LiteralPath $resolvedState -Force
        if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "Retry lab state cannot be a symlink or reparse point."
        }
    }
    return [pscustomobject]@{ Project = $Project; StateDirectory = $resolvedState }
}
