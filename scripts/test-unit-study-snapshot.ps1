. (Join-Path $PSScriptRoot 'unit-study-common.ps1')
$session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
Initialize-LedgerSyncLocalWebSession -Session $session
$identity = Invoke-LedgerSyncAcceptanceGET -Session $session -Path '/api/session'
$run = Invoke-LedgerSyncAcceptanceJSON -Session $session -Method POST -Path '/api/reconciliation/runs' -CSRFToken $identity.csrf_token -IdempotencyKey "snapshot-$([Guid]::NewGuid())" -Body @{} -ExpectedStatus 201
$detail = Invoke-LedgerSyncAcceptanceGET -Session $session -Path "/api/reconciliation/runs/$($run.Payload.run_id)"
Assert-LedgerSyncAcceptance ($detail.status -ceq 'matched' -and $detail.outbox_snapshot.isolation -ceq 'repeatable_read' -and $detail.ledger_watermark.Length -gt 0) 'Snapshot evidence is incomplete.'
$diagnostics = Invoke-LedgerSyncAcceptanceGET -Session $session -Path '/api/local/diagnostics'
Assert-LedgerSyncAcceptance ($null -ne $diagnostics.delivery_cache.redis.sampled_at -and $null -ne $diagnostics.delivery_cache.outbox.sampled_at) 'Service observation times are unavailable.'
$source = '10000000-0000-4000-8000-000000000001'
$path = "/api/accounts/$source/balance"
$before = Invoke-LedgerSyncAcceptanceGET -Session $session -Path $path
try {
    Invoke-LedgerSyncCompose -ComposeArguments @('stop','-t','1','redis') | Out-Null
    $cacheLoss = Invoke-LedgerSyncAcceptanceGET -Session $session -Path $path
    Assert-LedgerSyncAcceptance ($cacheLoss.ledger_minor -ceq $before.ledger_minor) 'Cache loss changed financial truth.'
} finally { Invoke-LedgerSyncCompose -ComposeArguments @('start','redis') | Out-Null }
try {
    Invoke-LedgerSyncCompose -ComposeArguments @('stop','-t','1','postgres') | Out-Null
    Start-Sleep -Seconds 2
    $unavailable = Invoke-WebRequest -UseBasicParsing -WebSession $session -Uri "$script:LedgerSyncWebUrl$path" -SkipHttpErrorCheck -TimeoutSec 15
    Assert-LedgerSyncAcceptance ([int]$unavailable.StatusCode -in @(503,504)) 'Database loss did not return an explicit unavailable result.'
} finally { Invoke-LedgerSyncCompose -ComposeArguments @('start','postgres') | Out-Null }
$deadline = [DateTime]::UtcNow.AddSeconds(30)
do { try { $recovered = Invoke-LedgerSyncAcceptanceGET -Session $session -Path $path; break } catch { if ([DateTime]::UtcNow -gt $deadline) {throw}; Start-Sleep -Milliseconds 500 } } while ($true)
Assert-LedgerSyncAcceptance ($recovered.ledger_minor -ceq $before.ledger_minor) 'Dependency recovery changed the balance.'
Invoke-LedgerSyncAcceptanceReconciliation -TenantID '00000000-0000-4000-8000-000000000001' | Out-Null
@{snapshot=$detail;service_observations=$diagnostics.delivery_cache;redis_loss='primary_balance_preserved';database_loss_status=[int]$unavailable.StatusCode;recovery='matched'} | ConvertTo-Json -Depth 9 | Set-Content -LiteralPath (Join-Path $studyRoot 'docs/release-evidence/unit-3-unit-4/snapshot-report.json')
Write-Host 'Snapshot counts, separate service times, cache fallback, and explicit database unavailability passed.'
