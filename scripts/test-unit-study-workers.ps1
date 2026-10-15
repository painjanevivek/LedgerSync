. (Join-Path $PSScriptRoot 'unit-study-common.ps1')
$ids = @(& docker ps --filter "label=com.docker.compose.project=$script:LedgerSyncComposeProject" --filter 'label=com.docker.compose.service=outbox-worker' --format '{{.ID}}')
Assert-LedgerSyncAcceptance ($ids.Count -eq 2) 'The parallel-worker check requires two isolated workers.'
$session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
Initialize-LedgerSyncLocalWebSession -Session $session
$identity = Invoke-LedgerSyncAcceptanceGET -Session $session -Path '/api/session'
for ($i=0; $i -lt 30; $i++) {
    Invoke-LedgerSyncAcceptanceTransfer -Session $session -CSRFToken $identity.csrf_token -SourceAccountID '10000000-0000-4000-8000-000000000001' -DestinationAccountID '10000000-0000-4000-8000-000000000002' -AmountMinor '100' -IdempotencyKey ([Guid]::NewGuid().ToString()) | Out-Null
}
$deadline = [DateTime]::UtcNow.AddSeconds(30)
do {
    $state = Invoke-LedgerSyncAcceptanceSQLJSON -SQL "SELECT jsonb_build_object('pending',(SELECT count(*) FROM outbox_events WHERE published_at IS NULL AND dead_at IS NULL),'dead',(SELECT count(*) FROM outbox_events WHERE dead_at IS NOT NULL),'workers',COALESCE((SELECT jsonb_agg(w) FROM (SELECT worker_id,count(*) AS published FROM outbox_publication_attempts WHERE status='published' GROUP BY worker_id) w),'[]'::jsonb));"
    if ([int]$state.pending -eq 0) { break }
    Start-Sleep -Milliseconds 250
} while ([DateTime]::UtcNow -lt $deadline)
Assert-LedgerSyncAcceptance ([int]$state.pending -eq 0 -and [int]$state.dead -eq 0 -and @($state.workers).Count -eq 2) 'Both workers must publish, and the queue must drain without dead events.'
Invoke-LedgerSyncAcceptanceReconciliation -TenantID '00000000-0000-4000-8000-000000000001' | Out-Null
@{captured_at=[DateTime]::UtcNow.ToString('o');containers=$ids;publication=$state;reconciliation='matched'} | ConvertTo-Json -Depth 7 | Set-Content -LiteralPath (Join-Path $studyRoot 'docs/release-evidence/unit-3-unit-4/parallel-workers.json')
Write-Host 'Both workers published committed events. The queue drained and reconciliation matched.'
