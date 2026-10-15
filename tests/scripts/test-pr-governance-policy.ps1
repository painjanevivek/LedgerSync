$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '../../scripts/pr-governance-policy.ps1')
$policy = [pscustomobject]@{
    schema_version = 1; repository = 'painjanevivek/LedgerSync'; solo_maintainer = 'painjanevivek'
    consolidation = [pscustomobject]@{
        pull_request = 23; branch = 'codex/portfolio-restructuring-r00-r10'; expires_at = '2026-10-31T23:59:59Z'
        maximum_review_files = 1200; maximum_critical_files = 400; maximum_visual_baselines = 120
        allowed_paths = @('README.md', 'internal/platform/db/postgres.go')
    }
}
$context = [pscustomobject]@{ repository = 'painjanevivek/LedgerSync'; author = 'painjanevivek'; number = 23; branch = 'codex/portfolio-restructuring-r00-r10' }
$body = "- [x] Solo-maintainer review applies under repository policy`n- [x] Consolidation scope exception 2026-10-ledgersync applies"
function Assert-PolicyCase([string]$Name, $CandidatePolicy, $CandidateContext, [string[]]$Files, [bool]$Solo, [bool]$Scope, [string]$CandidateBody = $body) {
    $actual = Resolve-LedgerSyncReviewPolicy -Policy $CandidatePolicy -Context $CandidateContext -ChangedFiles $Files -Body $CandidateBody
    if ($actual.SoloMaintainer -ne $Solo -or [bool]$actual.ScopeException -ne $Scope) { throw "Policy case failed: $Name" }
}
Assert-PolicyCase 'approved bounded consolidation' $policy $context @('README.md') $true $true
Assert-PolicyCase 'no base policy' $null $context @('README.md') $false $false
Assert-PolicyCase 'unlisted file' $policy $context @('unapproved.txt') $true $false
Assert-PolicyCase 'explicit acknowledgement required' $policy $context @('README.md') $false $false ''
$other = $context.PSObject.Copy(); $other.author = 'another-contributor'
Assert-PolicyCase 'author mismatch' $policy $other @('README.md') $false $false
$other = $context.PSObject.Copy(); $other.repository = 'another/repository'
Assert-PolicyCase 'repository mismatch' $policy $other @('README.md') $false $false
$other = $context.PSObject.Copy(); $other.number = 24
Assert-PolicyCase 'unrelated PR retains scope limit' $policy $other @('README.md') $true $false
$other = $context.PSObject.Copy(); $other.branch = 'another-branch'
Assert-PolicyCase 'branch mismatch' $policy $other @('README.md') $true $false
$expired = $policy | ConvertTo-Json -Depth 6 | ConvertFrom-Json; $expired.consolidation.expires_at = '2020-01-01T00:00:00Z'
Assert-PolicyCase 'expired scope exception' $expired $context @('README.md') $true $false
$excessive = $policy | ConvertTo-Json -Depth 6 | ConvertFrom-Json; $excessive.consolidation.maximum_review_files = 1201
Assert-PolicyCase 'unbounded scope rejected' $excessive $context @('README.md') $true $false
$selfApproval = $policy | ConvertTo-Json -Depth 6 | ConvertFrom-Json; $selfApproval.consolidation.allowed_paths += 'scripts/pr-governance-policy.ps1'
Assert-PolicyCase 'exception cannot change its own guard' $selfApproval $context @('scripts/pr-governance-policy.ps1') $true $false
Write-Host 'Review-policy exception checks passed: 11 cases.'
exit 0
