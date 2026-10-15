[CmdletBinding()]
param([switch]$Browser)
. (Join-Path $PSScriptRoot 'unit-study-common.ps1')
$evidenceRoot = Join-Path $studyRoot 'docs/release-evidence/unit-3-unit-4'
New-Item -ItemType Directory -Force -Path $evidenceRoot | Out-Null
$manifest = @(& git -C $studyRoot ls-files --cached --others --exclude-standard) | Sort-Object -Unique | ForEach-Object {
    $path = Join-Path $studyRoot $_
    if ((Test-Path -LiteralPath $path -PathType Leaf) -and $_ -notmatch '^docs/release-evidence/unit-3-unit-4/') {
        [ordered]@{ path = $_; sha256 = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() }
    }
}
$sourceJSON = $manifest | ConvertTo-Json -Depth 3 -Compress
$fingerprint = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($sourceJSON))).ToLowerInvariant()
$images = @(& docker ps -a --filter "label=com.docker.compose.project=$script:LedgerSyncComposeProject" --format '{{.ID}}|{{.Image}}') | ForEach-Object {
    $parts = $_.Split('|')
    @{ container = $parts[0]; image = $parts[1]; image_id = (& docker inspect --format '{{.Image}}' $parts[0]).Trim() }
}
$session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
Initialize-LedgerSyncLocalWebSession -Session $session
$identity = Invoke-LedgerSyncAcceptanceGET -Session $session -Path '/api/session'
$transferID = Test-LedgerSyncAcceptanceJourney -Session $session -CSRFToken $identity.csrf_token -AmountMinor '100' -VerifyReplay
$summary = Invoke-LedgerSyncAcceptanceReconciliation -TenantID '00000000-0000-4000-8000-000000000001'
$record = [ordered]@{
    captured_at_utc = [DateTime]::UtcNow.ToString('o'); commit = (& git -C $studyRoot rev-parse HEAD).Trim()
    source_fingerprint_sha256 = $fingerprint; dirty_paths = @(& git -C $studyRoot status --porcelain)
    project = $script:LedgerSyncComposeProject; images = $images
    docker_resources = (& docker info --format '{{json .}}' | ConvertFrom-Json | Select-Object NCPU, MemTotal, ServerVersion)
    operational_summary = $summary; financial_journey = 'passed'; replay = 'passed'; reconciliation = 'matched'
    financial_transfer_id = $transferID; browser = 'not_run'; source_manifest = $manifest
}
$record | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $evidenceRoot 'baseline.json')
if ($Browser) {
    Export-LedgerSyncStudyEnvironment
    Push-Location (Join-Path $studyRoot 'web')
    try {
        & npm run test:e2e:real-stack -- --workers=1 2>&1 | Tee-Object -FilePath (Join-Path $evidenceRoot 'baseline-browser.txt')
        if ($LASTEXITCODE -ne 0) { throw 'The isolated baseline browser checks failed.' }
        $record.browser = 'passed'
        $record | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $evidenceRoot 'baseline.json')
    } finally { Pop-Location }
}
Write-Host 'Baseline financial journey, replay, and reconciliation passed.'
