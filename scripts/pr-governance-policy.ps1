function Resolve-LedgerSyncReviewPolicy {
    param($Policy, $Context, [string[]]$ChangedFiles, [string]$Body)
    $result = [pscustomobject]@{ SoloMaintainer = $false; ScopeException = $null }
    if (-not $Policy -or -not $Context -or $Policy.schema_version -ne 1) { return $result }
    if ($Context.repository -cne $Policy.repository -or $Context.author -cne $Policy.solo_maintainer) { return $result }
    $result.SoloMaintainer = $Body -match '(?im)^- \[[xX]\] Solo-maintainer review applies under repository policy\s*$'
    $exception = $Policy.consolidation
    if (-not $exception -or $Context.number -ne $exception.pull_request -or $Context.branch -cne $exception.branch) { return $result }
    if ($Body -notmatch '(?im)^- \[[xX]\] Consolidation scope exception 2026-10-ledgersync applies\s*$') { return $result }
    $expiry = [DateTimeOffset]$exception.expires_at
    if ($expiry -le [DateTimeOffset]::UtcNow) { return $result }
    if ($exception.maximum_review_files -lt 60 -or $exception.maximum_review_files -gt 1200 -or
        $exception.maximum_critical_files -lt 30 -or $exception.maximum_critical_files -gt 400 -or
        $exception.maximum_visual_baselines -lt 60 -or $exception.maximum_visual_baselines -gt 120) { return $result }
    $allowed = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($path in $exception.allowed_paths) { [void]$allowed.Add([string]$path) }
    foreach ($path in $ChangedFiles) {
        if (-not $allowed.Contains($path) -or $path -match '^\.github/governance/|^scripts/(check-pr-governance|pr-governance-policy)\.ps1$|^tests/scripts/test-pr-governance') { return $result }
    }
    $result.ScopeException = $exception
    return $result
}

function Get-LedgerSyncApprovedReviewPolicy {
    param([string]$RepositoryRoot, [string[]]$ChangedFiles, [string]$Body)
    # In CI, only policy already present on the trusted base can grant an
    # exception. A pull request cannot approve its own policy document.
    if (-not $env:GITHUB_EVENT_PATH) { return [pscustomobject]@{ SoloMaintainer = $false; ScopeException = $null } }
    $event = Get-Content -LiteralPath $env:GITHUB_EVENT_PATH -Raw | ConvertFrom-Json
    $base = [string]$event.pull_request.base.sha
    if ($base -cnotmatch '^[0-9a-f]{40}$') { throw 'A trusted pull-request base SHA is required.' }
    $policyText = @(& git -C $RepositoryRoot show "${base}:.github/governance/merge-policy.json" 2>$null)
    $policyExit = $LASTEXITCODE
    $global:LASTEXITCODE = 0
    if ($policyExit -ne 0) { return [pscustomobject]@{ SoloMaintainer = $false; ScopeException = $null } }
    $policy = ($policyText -join "`n") | ConvertFrom-Json
    $context = [pscustomobject]@{
        repository = [string]$event.repository.full_name
        author = [string]$event.pull_request.user.login
        number = [int]$event.number
        branch = [string]$event.pull_request.head.ref
    }
    return Resolve-LedgerSyncReviewPolicy -Policy $policy -Context $context -ChangedFiles $ChangedFiles -Body $Body
}
