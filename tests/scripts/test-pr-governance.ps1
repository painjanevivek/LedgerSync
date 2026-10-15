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
        [int]$MaximumCriticalFiles = 30,
        [int]$MaximumVisualBaselineFiles = 60
    )

    $filesPath = Join-Path $testRoot "$Name-files.txt"
    $bodyPath = Join-Path $testRoot "$Name-body.md"
    Set-Content -LiteralPath $filesPath -Value $Files
    Set-Content -LiteralPath $bodyPath -Value $Body

    & $powerShellExecutable -NoLogo -NoProfile -NonInteractive -File $governanceScript `
        -ChangedFilesPath $filesPath `
        -PullRequestBodyPath $bodyPath `
        -MaximumChangedFiles $MaximumChangedFiles `
        -MaximumCriticalFiles $MaximumCriticalFiles `
        -MaximumVisualBaselineFiles $MaximumVisualBaselineFiles *> $null
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
    Invoke-GovernanceCase -Name "reviewed-visual-baselines" -Files @(
        "docs/design/qa/responsive/baseline-approvals.md",
        "docs/design/qa/responsive/baselines/linux/chromium/overview.png",
        "docs/design/qa/responsive/baselines/linux/chromium/accounts.png"
    ) -Body $completeBody -MaximumChangedFiles 1 -ExpectedExitCode 0
    Invoke-GovernanceCase -Name "unreviewed-visual-baseline" -Files @(
        "docs/design/qa/responsive/baselines/linux/chromium/overview.png"
    ) -Body $completeBody -ExpectedExitCode 1
    Invoke-GovernanceCase -Name "oversized-visual-baselines" -Files @(
        "docs/design/qa/responsive/baseline-approvals.md",
        "docs/design/qa/responsive/baselines/linux/chromium/overview.png",
        "docs/design/qa/responsive/baselines/linux/chromium/accounts.png"
    ) -Body $completeBody -MaximumVisualBaselineFiles 1 -ExpectedExitCode 1
    Invoke-GovernanceCase -Name "unacknowledged-critical" -Files @("migrations/000034_example.up.sql") -Body ($completeBody -replace '\[x\] Financial/security behavior changed', '[ ] Financial/security behavior changed') -ExpectedExitCode 1
    # Exercise the complete child guard, not only its pure policy resolver.
    # This catches PowerShell ValidateRange attributes on reassigned variables.
    $originalEventPath = $env:GITHUB_EVENT_PATH
    try {
        $policy = Get-Content -LiteralPath (Join-Path $repositoryRoot '.github/governance/merge-policy.json') -Raw | ConvertFrom-Json
        $base = (& git -C $repositoryRoot rev-parse HEAD).Trim()
        $eventPath = Join-Path $testRoot 'bounded-scope-event.json'
        @{
            number = 23
            repository = @{ full_name = 'painjanevivek/LedgerSync' }
            pull_request = @{
                user = @{ login = 'painjanevivek' }
                base = @{ sha = $base }
                head = @{ ref = 'codex/portfolio-restructuring-r00-r10' }
            }
        } | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $eventPath
        $env:GITHUB_EVENT_PATH = $eventPath
        $scopeFiles = @($policy.consolidation.allowed_paths | Where-Object { $_ -notmatch '^docs/design/qa/responsive/baselines/' } | Select-Object -First 100)
        $scopeBody = ($completeBody -replace 'The author is not the sole approving reviewer', 'Solo-maintainer review applies under repository policy') + "`n- [x] Consolidation scope exception 2026-10-ledgersync applies`n"
        # Once this one-time policy expires, the same oversized request must fail.
        $scopeExit = if ([DateTimeOffset]$policy.consolidation.expires_at -gt [DateTimeOffset]::UtcNow) { 0 } else { 1 }
        Invoke-GovernanceCase -Name 'approved-bounded-scope' -Files $scopeFiles -Body $scopeBody -ExpectedExitCode $scopeExit
        Invoke-GovernanceCase -Name 'unlisted-bounded-scope' -Files ($scopeFiles + 'unapproved/extra.go') -Body $scopeBody -ExpectedExitCode 1
        Invoke-GovernanceCase -Name 'unacknowledged-bounded-scope' -Files $scopeFiles -Body ($scopeBody -replace '\[x\] Consolidation', '[ ] Consolidation') -ExpectedExitCode 1
    }
    finally {
        if ($null -eq $originalEventPath) { Remove-Item Env:GITHUB_EVENT_PATH -ErrorAction SilentlyContinue }
        else { $env:GITHUB_EVENT_PATH = $originalEventPath }
    }
    & $powerShellExecutable -NoLogo -NoProfile -NonInteractive -File (Join-Path $PSScriptRoot "test-pr-governance-policy.ps1")
    if ($LASTEXITCODE -ne 0) { throw "Review-policy exception checks failed" }
    Write-Host "PR governance checks passed."
}
finally {
    Remove-Item -LiteralPath $testRoot -Recurse -Force -ErrorAction SilentlyContinue
}

# The final negative fixture intentionally leaves LASTEXITCODE at 1. Normalize
# the process result after every assertion has passed so CI sees the suite's
# outcome instead of the expected child-process failure.
exit 0
