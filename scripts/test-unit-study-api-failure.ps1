. (Join-Path $PSScriptRoot 'unit-study-common.ps1')
$containerIDs = @(& docker ps --filter "label=com.docker.compose.project=$script:LedgerSyncComposeProject" --filter 'label=com.docker.compose.service=api' --format '{{.ID}}')
Assert-LedgerSyncAcceptance ($containerIDs.Count -eq 2) 'The API failure drill requires two isolated API instances.'
$target = [string]$containerIDs[0]
Assert-LedgerSyncAcceptance ($target -cmatch '^[0-9a-f]{12,64}$') 'The failure target is not an exact container identity.'
$probe = @'
(async()=>{ const seen=new Set();for(let i=0;i<20;i++){ const r=await fetch('http://study-proxy:8080/healthz');if(!r.ok)throw Error();seen.add(r.headers.get('x-ledgersync-node')) }
if(seen.size!==2||seen.has(null))throw Error();console.log(JSON.stringify({ready_nodes:[...seen]}));})().catch(()=>{console.error('Balanced routing check failed');process.exit(1)});
'@
$routing = @(Invoke-LedgerSyncCompose -ComposeArguments @('exec','-T','web','node','-e',$probe) -CaptureOutput) -join "`n"
$job = Start-Job -ArgumentList $PSScriptRoot -ScriptBlock {
    param($scriptRoot)
    . (Join-Path $scriptRoot 'unit-study-common.ps1')
    $session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
    Initialize-LedgerSyncLocalWebSession -Session $session
    $identity = Invoke-LedgerSyncAcceptanceGET -Session $session -Path '/api/session'
    $observations = @()
    for ($i=0; $i -lt 40; $i++) {
        $key = [Guid]::NewGuid().ToString()
        $body = @{ sourceAccountId='10000000-0000-4000-8000-000000000001'; destinationAccountId='10000000-0000-4000-8000-000000000002'; amount=@{currency='INR';minorUnits='100'} }
        $attempts = @()
        $committed = $false
        for ($attempt=0; $attempt -lt 4; $attempt++) {
            $at = [DateTime]::UtcNow.ToString('o')
            try {
                $response = Invoke-LedgerSyncAcceptanceJSON -Session $session -Method POST -Path '/api/transfers' -CSRFToken $identity.csrf_token -IdempotencyKey $key -Body $body -ExpectedStatus @(201,503,504)
                $attempts += @{at=$at;status=[int]$response.Status}
                if ([int]$response.Status -eq 201) { $committed=$true; $transferID=[string]$response.Payload.transfer_id; break }
            } catch { $attempts += @{at=$at;status='interrupted'} }
            Start-Sleep -Milliseconds 300
        }
        if (-not $committed) { throw 'A study command did not resolve within its bounded retry limit.' }
        $observations += @{key=$key;transfer_id=$transferID;attempts=$attempts}
        Start-Sleep -Milliseconds 100
    }
    $observations | ConvertTo-Json -Depth 6 -Compress
}
try {
    Start-Sleep -Seconds 2
    $stoppedAt = [DateTime]::UtcNow
    & docker stop --time 1 $target | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Could not stop the exact isolated API target.' }
    $deadline = [DateTime]::UtcNow.AddSeconds(15)
    do {
        $state = @(Invoke-LedgerSyncCompose -ComposeArguments @('exec','-T','web','node','-e',"fetch('http://study-proxy:8080/internal/study/replicas').then(r=>r.text()).then(console.log)") -CaptureOutput) -join '' | ConvertFrom-Json
        if ([int]$state.ready_api_count -eq 1) { break }
        Start-Sleep -Milliseconds 200
    } while ([DateTime]::UtcNow -lt $deadline)
    Assert-LedgerSyncAcceptance ([int]$state.ready_api_count -eq 1) 'The gateway did not detect API failure within 15 seconds.'
    $recoveryMilliseconds = ([DateTime]::UtcNow - $stoppedAt).TotalMilliseconds
    $null = Wait-Job -Job $job -Timeout 60
    if ($job.State -ne 'Completed') { throw 'The financial workload failed or exceeded its deadline.' }
    $observations = @(Receive-Job -Job $job -ErrorAction Stop) -join '' | ConvertFrom-Json
    Assert-LedgerSyncAcceptance (@($observations).Count -eq 40) 'The failure workload did not complete all commands.'
    $sql = "SELECT json_build_object('unbalanced', (SELECT count(*) FROM (SELECT journal_transaction_id FROM ledger_postings GROUP BY journal_transaction_id HAVING sum(CASE WHEN direction='credit' THEN amount_minor::numeric ELSE -amount_minor::numeric END) <> 0) b), 'duplicate_transfer_journals',(SELECT count(*) FROM (SELECT transfer_id FROM journal_transactions WHERE transfer_id IS NOT NULL GROUP BY transfer_id HAVING count(*) > 1) d));"
    $truth = Invoke-LedgerSyncAcceptanceSQLJSON -SQL $sql
    Assert-LedgerSyncAcceptance ([int]$truth.unbalanced -eq 0 -and [int]$truth.duplicate_transfer_journals -eq 0) 'The failure drill violated financial correctness.'
    Invoke-LedgerSyncAcceptanceReconciliation -TenantID '00000000-0000-4000-8000-000000000001' | Out-Null
    @{captured_at=[DateTime]::UtcNow.ToString('o');routing=($routing|ConvertFrom-Json);failed_container=$target;detection_milliseconds=$recoveryMilliseconds;requests=$observations;financial_checks=$truth;reconciliation='matched'} | ConvertTo-Json -Depth 9 | Set-Content -LiteralPath (Join-Path $studyRoot 'docs/release-evidence/unit-3-unit-4/api-failure.json')
} finally {
    if ($job.State -eq 'Running') { Stop-Job -Job $job }
    Remove-Job -Job $job -Force
    & docker start $target | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'The isolated API target could not be restored.' }
}
Write-Host 'Both APIs served requests. The failure drill resolved all commands and reconciliation matched.'
