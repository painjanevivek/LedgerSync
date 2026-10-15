. (Join-Path $PSScriptRoot 'unit-study-common.ps1')
$tenant = '00000000-0000-4000-8000-000000000001'
function Get-StudyScheduler {
    Invoke-LedgerSyncAcceptanceSQLJSON -SQL "SELECT jsonb_build_object('lease',(SELECT jsonb_build_object('owner_id',owner_id,'generation',generation::text,'expires_at',expires_at,'active',expires_at>clock_timestamp()) FROM reconciliation_scheduler_leases WHERE tenant_id='$tenant'),'completed',(SELECT count(*) FROM reconciliation_scheduled_jobs WHERE tenant_id='$tenant' AND state='completed'),'duplicate_intervals',(SELECT count(*) FROM (SELECT scheduled_for FROM reconciliation_scheduled_jobs WHERE tenant_id='$tenant' GROUP BY scheduled_for HAVING count(*)>1) j),'mismatches',(SELECT count(*) FROM reconciliation_scheduled_jobs WHERE tenant_id='$tenant' AND outcome<>'matched'));"
}
$deadline = [DateTime]::UtcNow.AddSeconds(30)
do { $first = Get-StudyScheduler; if ($first.lease.active -and [int]$first.completed -gt 0) {break}; Start-Sleep -Milliseconds 250 } while ([DateTime]::UtcNow -lt $deadline)
Assert-LedgerSyncAcceptance ($first.lease.active -and [int]$first.completed -gt 0) 'A restricted scheduler did not complete reconciliation.'
$ids = @(Invoke-LedgerSyncCompose -ComposeArguments @('--profile','scheduler','ps','-q','reconciliation-scheduler') -CaptureOutput)
Assert-LedgerSyncAcceptance ($ids.Count -eq 2) 'Two scheduler instances are required.'
$nodes = @{}
foreach ($id in $ids) { $hostname = (& docker inspect --format '{{.Config.Hostname}}' $id).Trim(); if ($LASTEXITCODE -ne 0) {throw 'Scheduler identity inspection failed.'}; $nodes["ledgersync/scheduler/$hostname"]=$id }
$drills = @()
foreach ($mode in @('stop','pause')) {
    $before = Get-StudyScheduler
    $leader = $nodes[$before.lease.owner_id]
    Assert-LedgerSyncAcceptance (-not [string]::IsNullOrWhiteSpace($leader)) 'Lease owner is outside the verified scheduler containers.'
    $watch = [Diagnostics.Stopwatch]::StartNew()
    try {
        if ($mode -ceq 'stop') { & docker stop --time 1 $leader | Out-Null } else { & docker pause $leader | Out-Null }
        if ($LASTEXITCODE -ne 0) {throw 'Isolated scheduler failure injection failed.'}
        $deadline = [DateTime]::UtcNow.AddSeconds(15)
        do { $after = Get-StudyScheduler; if ($after.lease.active -and $after.lease.owner_id -cne $before.lease.owner_id -and [int64]$after.lease.generation -gt [int64]$before.lease.generation) {break}; Start-Sleep -Milliseconds 200 } while ([DateTime]::UtcNow -lt $deadline)
        $watch.Stop()
        Assert-LedgerSyncAcceptance ($after.lease.active -and $after.lease.owner_id -cne $before.lease.owner_id -and [int64]$after.lease.generation -gt [int64]$before.lease.generation) 'Scheduler takeover failed.'
        $staleSQL = "SELECT public.claim_reconciliation_schedule('$tenant','$($before.lease.owner_id)',$($before.lease.generation),15);"
        $probe = 'export PGPASSWORD="$LEDGERSYNC_SCHEDULER_DATABASE_PASSWORD"; exec psql -v ON_ERROR_STOP=1 -h postgres -U ledgersync_local_scheduler -d ledgersync -c "$1"'
        $staleRejected = $false
        try { Invoke-LedgerSyncCompose -ComposeArguments @('run','--rm','--no-deps','--entrypoint','sh','migrate','-ec',$probe,'study-fence',$staleSQL) -CaptureOutput | Out-Null } catch { $staleRejected = $_.Exception.Message.Contains('stale scheduler fence') }
        Assert-LedgerSyncAcceptance $staleRejected 'A stale scheduler could enqueue work.'
        $drills += @{mode=$mode;before=$before.lease;after=$after.lease;takeover_ms=$watch.ElapsedMilliseconds;stale_owner_rejected=$staleRejected}
    } finally {
        if ($mode -ceq 'stop') {& docker start $leader | Out-Null} else {& docker unpause $leader | Out-Null}
        if ($LASTEXITCODE -ne 0) {throw 'Scheduler restore failed.'}
    }
}
Invoke-LedgerSyncAcceptanceReconciliation -TenantID $tenant | Out-Null
$final = Get-StudyScheduler
Assert-LedgerSyncAcceptance ([int]$final.duplicate_intervals -eq 0 -and [int]$final.mismatches -eq 0) 'Scheduled reports have duplicate intervals or mismatches.'
@{captured_at=[DateTime]::UtcNow.ToString('o');drills=$drills;final=$final;manual_reconciliation='matched'} | ConvertTo-Json -Depth 9 | Set-Content -LiteralPath (Join-Path $studyRoot 'docs/release-evidence/unit-3-unit-4/scheduler-takeover.json')
Write-Host 'Scheduler stop and pause takeover, stale fences, interval uniqueness, and manual reconciliation passed.'
