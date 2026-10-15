[CmdletBinding()]
param()
. (Join-Path $PSScriptRoot 'unit-study-common.ps1')
$evidenceRoot = Join-Path $studyRoot 'docs/release-evidence/unit-3-unit-4'
$session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
Initialize-LedgerSyncLocalWebSession -Session $session
$identity = Invoke-LedgerSyncAcceptanceGET -Session $session -Path '/api/session'
$transferID = Test-LedgerSyncAcceptanceJourney -Session $session -CSRFToken $identity.csrf_token -AmountMinor '100' -VerifyReplay
$summary = Invoke-LedgerSyncAcceptanceReconciliation -TenantID '00000000-0000-4000-8000-000000000001'
$images = @(& docker ps -a --filter "label=com.docker.compose.project=$script:LedgerSyncComposeProject" --format '{{.ID}}|{{.Image}}') | ForEach-Object {
    $parts = $_.Split('|')
    [ordered]@{ container = $parts[0]; image = $parts[1]; image_id = (& docker inspect --format '{{.Image}}' $parts[0]).Trim() }
}
$manifest = @(& git -C $studyRoot ls-files --cached --others --exclude-standard) | Sort-Object -Unique | ForEach-Object {
    $path = Join-Path $studyRoot $_
    if ((Test-Path -LiteralPath $path -PathType Leaf) -and $_ -notmatch '^docs/release-evidence/unit-3-unit-4/') {
        [ordered]@{ path = $_; sha256 = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() }
    }
}
$sourceJSON = $manifest | ConvertTo-Json -Depth 3 -Compress
$fingerprint = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($sourceJSON))).ToLowerInvariant()
[ordered]@{
    captured_at_utc = [DateTime]::UtcNow.ToString('o')
    commit = (& git -C $studyRoot rev-parse HEAD).Trim()
    source_fingerprint_sha256 = $fingerprint
    project = $script:LedgerSyncComposeProject
    images = $images
    operational_summary = $summary
    financial_journey = 'passed'
    replay = 'passed'
    reconciliation = 'matched'
    financial_transfer_id = $transferID
} | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $evidenceRoot 'final-acceptance.json')
Write-Host 'Final isolated financial journey, identical replay, and reconciliation passed.'
