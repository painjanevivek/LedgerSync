. (Join-Path $PSScriptRoot 'unit-study-common.ps1')
$session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
Initialize-LedgerSyncLocalWebSession -Session $session
$identity = Invoke-LedgerSyncAcceptanceGET -Session $session -Path '/api/session'
$transferID = Test-LedgerSyncAcceptanceJourney -Session $session -CSRFToken $identity.csrf_token -AmountMinor '100' -VerifyReplay
$deadline = [DateTime]::UtcNow.AddSeconds(20)
do {
    $trace = Invoke-LedgerSyncAcceptanceSQLJSON -SQL "SELECT jsonb_build_object('transfer_id','$transferID','origin',COALESCE((SELECT jsonb_agg(e) FROM (SELECT id,node_id,logical_timestamp::text,parent_reference FROM outbox_events WHERE transfer_id='$transferID'::uuid) e),'[]'::jsonb),'publication',COALESCE((SELECT jsonb_agg(a) FROM (SELECT a.event_id,a.node_id,a.receive_counter::text,a.send_counter::text,a.status FROM outbox_publication_attempts a JOIN outbox_events e ON e.id=a.event_id WHERE e.transfer_id='$transferID'::uuid) a),'[]'::jsonb),'receipts',COALESCE((SELECT jsonb_agg(r) FROM (SELECT r.event_id,r.node_id,r.sender_counter::text,r.receive_counter::text,r.received_at FROM logical_message_receipts r JOIN outbox_events e ON e.id=r.event_id WHERE e.transfer_id='$transferID'::uuid) r),'[]'::jsonb),'invalid_order',(SELECT count(*) FROM outbox_publication_attempts a JOIN outbox_events e ON e.id=a.event_id WHERE e.transfer_id='$transferID'::uuid AND (a.receive_counter<=e.logical_timestamp OR a.send_counter<=a.receive_counter)));"
    if (@($trace.receipts).Count -ge 2) { break }
    Start-Sleep -Milliseconds 200
} while ([DateTime]::UtcNow -lt $deadline)
Assert-LedgerSyncAcceptance (@($trace.origin).Count -ge 2 -and @($trace.publication).Count -ge 2 -and @($trace.receipts).Count -ge 2 -and [int]$trace.invalid_order -eq 0) 'The causal trace is incomplete or has an invalid order.'
foreach ($event in $trace.origin) {
    Assert-LedgerSyncAcceptance (-not [string]::IsNullOrWhiteSpace($event.node_id) -and [int64]$event.logical_timestamp -gt 0) 'Origin metadata is unavailable for a new API event.'
    $detail = Invoke-LedgerSyncAcceptanceGET -Session $session -Path "/api/events/$($event.id)"
    Assert-LedgerSyncAcceptance ([string]$detail.event_order.logical_timestamp -ceq [string]$event.logical_timestamp) 'Authorized event details changed the exact logical timestamp.'
}
foreach ($receipt in $trace.receipts) { Assert-LedgerSyncAcceptance ([int64]$receipt.receive_counter -gt [int64]$receipt.sender_counter) 'A receive counter did not exceed its send counter.' }
Invoke-LedgerSyncAcceptanceReconciliation -TenantID '00000000-0000-4000-8000-000000000001' | Out-Null
@{captured_at=[DateTime]::UtcNow.ToString('o');trace=$trace;reconciliation='matched'} | ConvertTo-Json -Depth 9 | Set-Content -LiteralPath (Join-Path $studyRoot 'docs/release-evidence/unit-3-unit-4/causal-trace.json')
Write-Host 'API origin, worker publication, and projection receive clocks passed. Reconciliation matched.'
