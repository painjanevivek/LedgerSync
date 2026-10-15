. (Join-Path $PSScriptRoot 'unit-study-common.ps1')
$driver = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'study-private-commands.mjs') -Raw
$output = Invoke-LedgerSyncCompose -ComposeArguments @('exec','-T','web','node','--input-type=module','-e',$driver) -CaptureOutput
$result = ($output | Where-Object { $_ -match '^\{' } | Select-Object -Last 1) | ConvertFrom-Json
Assert-LedgerSyncAcceptance (@($result.requests.node | Sort-Object -Unique).Count -eq 2) 'Both APIs must serve the concurrent commands.'
$hot = $result.accounts.hot
$check = Invoke-LedgerSyncAcceptanceSQLJSON -SQL "SELECT jsonb_build_object('hot_balance',(SELECT ledger_minor::text FROM account_balance_projections WHERE account_id='$hot'::uuid),'hot_posted',(SELECT count(*) FROM transfers WHERE debit_account_id='$hot'::uuid AND status='posted'),'same_key_journals',(SELECT count(*) FROM journal_transactions WHERE id=(SELECT journal_transaction_id FROM transfers WHERE id='$($result.same_key_transfer)'::uuid)),'unbalanced',(SELECT count(*) FROM (SELECT journal_transaction_id FROM ledger_postings GROUP BY journal_transaction_id HAVING count(*)<>2 OR sum(CASE WHEN direction='credit' THEN amount_minor::numeric ELSE -amount_minor::numeric END)<>0) j));"
Assert-LedgerSyncAcceptance ($check.hot_balance -ceq '0' -and [int]$check.hot_posted -eq 10 -and [int]$check.same_key_journals -eq 1 -and [int]$check.unbalanced -eq 0) 'Concurrent financial invariants failed. Stop acceptance.'
Invoke-LedgerSyncAcceptanceReconciliation -TenantID '00000000-0000-4000-8000-000000000001' | Out-Null
@{workload=$result;database=$check;reconciliation='matched'} | ConvertTo-Json -Depth 9 | Set-Content -LiteralPath (Join-Path $studyRoot 'docs/release-evidence/unit-3-unit-4/concurrent-transfers.json')
Write-Host 'Concurrent debits, opposite transfers, shared idempotency, and reconciliation passed through both APIs.'
