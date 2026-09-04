[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string]$ChangedFilesPath,
    [Parameter(Mandatory)]
    [string]$PullRequestBodyPath,
    [ValidateRange(1, 500)]
    [int]$MaximumChangedFiles = 60,
    [ValidateRange(1, 500)]
    [int]$MaximumCriticalFiles = 30,
    [ValidateRange(1, 500)]
    [int]$MaximumVisualBaselineFiles = 60
)

$ErrorActionPreference = "Stop"

foreach ($path in @($ChangedFilesPath, $PullRequestBodyPath)) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "Required governance input does not exist: $path"
    }
}

$changedFiles = @(
    Get-Content -LiteralPath $ChangedFilesPath |
        ForEach-Object { $_.Trim().Replace("\", "/") } |
        Where-Object { $_ -ne "" } |
        Sort-Object -Unique
)
$body = Get-Content -LiteralPath $PullRequestBodyPath -Raw

$criticalPathPattern = '^(migrations/|deploy/postgres/|contracts/|\.github/workflows/|internal/domain/|internal/platform/(db|identity)/|internal/application/(transfers|funding|corrections)/|web/src/lib/(oidc|actor-assertion)\.ts$)'
$visualBaselinePattern = '^docs/design/qa/responsive/baselines/(linux|windows)/chromium/.+\.png$'
$visualBaselineFiles = @($changedFiles | Where-Object { $_ -match $visualBaselinePattern })
$reviewFiles = @($changedFiles | Where-Object { $_ -notmatch $visualBaselinePattern })
$criticalFiles = @($reviewFiles | Where-Object { $_ -match $criticalPathPattern })
$errors = [System.Collections.Generic.List[string]]::new()

if ($changedFiles.Count -eq 0) {
    $errors.Add("The pull request contains no changed-file evidence.")
}
if ($reviewFiles.Count -gt $MaximumChangedFiles) {
    $errors.Add("The pull request changes $($reviewFiles.Count) review files; the review limit is $MaximumChangedFiles. Split it or document an approved exception in a dedicated governance change.")
}
if ($visualBaselineFiles.Count -gt $MaximumVisualBaselineFiles) {
    $errors.Add("The pull request changes $($visualBaselineFiles.Count) visual baselines; the baseline review limit is $MaximumVisualBaselineFiles.")
}
if ($visualBaselineFiles.Count -gt 0 -and $changedFiles -notcontains 'docs/design/qa/responsive/baseline-approvals.md') {
    $errors.Add("Visual baseline changes require an entry in docs/design/qa/responsive/baseline-approvals.md.")
}
if ($criticalFiles.Count -gt $MaximumCriticalFiles) {
    $errors.Add("The pull request changes $($criticalFiles.Count) critical files; the critical review limit is $MaximumCriticalFiles.")
}

$requiredSections = @("Risk", "Test evidence", "Rollout and rollback", "Reviewer independence")
foreach ($section in $requiredSections) {
    if ($body -notmatch "(?im)^##\s+$([regex]::Escape($section))\s*$") {
        $errors.Add("Pull request body is missing the required section: ## $section")
    }
}

if ($criticalFiles.Count -gt 0) {
    if ($body -notmatch '(?im)^- \[[xX]\] (Financial/security behavior changed|Database schema or grants changed|Public API, event, or stored idempotency contract changed|Worker ownership, retry, lease, or delivery behavior changed)\s*$') {
        $errors.Add("Critical paths changed, but no applicable risk checkbox is selected.")
    }
    if ($body -notmatch '(?im)^- \[[xX]\] The author is not the sole approving reviewer\s*$') {
        $errors.Add("Critical paths changed, but independent review is not acknowledged.")
    }
}

if ($errors.Count -gt 0) {
    $errors | ForEach-Object { Write-Error $_ }
    exit 1
}

Write-Host "Governance evidence accepted: review_files=$($reviewFiles.Count), visual_baselines=$($visualBaselineFiles.Count), critical=$($criticalFiles.Count)."
