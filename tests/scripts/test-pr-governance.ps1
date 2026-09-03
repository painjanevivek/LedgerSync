[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$governanceScript = Join-Path $repositoryRoot "scripts/check-pr-governance.ps1"
$powerShellExecutable = (Get-Process -Id $PID).Path
$testRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("ledgersync-governance-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $testRoot | Out-Null

function Invoke-GovernanceCase {
    param(
        [string]$Name,
        [string[]]$Files,
        [string]$Body,
        [int]$ExpectedExitCode,
        [int]$MaximumChangedFiles = 60,
        [int]$MaximumCriticalFiles = 30
    )

    $filesPath = Join-Path $testRoot "$Name-files.txt"
    $bodyPath = Join-Path $testRoot "$Name-body.md"
    Set-Content -LiteralPath $filesPath -Value $Files
    Set-Content -LiteralPath $bodyPath -Value $Body

    & $powerShellExecutable -NoLogo -NoProfile -NonInteractive -File $governanceScript `
        -ChangedFilesPath $filesPath `
        -PullRequestBodyPath $bodyPath `
        -MaximumChangedFiles $MaximumChangedFiles `
        -MaximumCriticalFiles $MaximumCriticalFiles *> $null
    if ($LASTEXITCODE -ne $ExpectedExitCode) {
        throw "$Name expected exit code $ExpectedExitCode but received $LASTEXITCODE"
    }
}

$completeBody = @'
## Risk
- [x] Financial/security behavior changed
## Test evidence
Evidence
## Rollout and rollback
Steps
## Reviewer independence
- [x] The author is not the sole approving reviewer
'@

try {
    Invoke-GovernanceCase -Name "valid-critical" -Files @("internal/platform/db/postgres.go", "tests/unit/example_test.go") -Body $completeBody -ExpectedExitCode 0
    Invoke-GovernanceCase -Name "missing-sections" -Files @("README.md") -Body "## Risk" -ExpectedExitCode 1
    Invoke-GovernanceCase -Name "oversized" -Files @("README.md", "docs/one.md") -Body $completeBody -MaximumChangedFiles 1 -ExpectedExitCode 1
    Invoke-GovernanceCase -Name "unacknowledged-critical" -Files @("migrations/000034_example.up.sql") -Body ($completeBody -replace '\[x\] Financial/security behavior changed', '[ ] Financial/security behavior changed') -ExpectedExitCode 1
    Write-Host "PR governance checks passed."
}
finally {
    Remove-Item -LiteralPath $testRoot -Recurse -Force -ErrorAction SilentlyContinue
}
