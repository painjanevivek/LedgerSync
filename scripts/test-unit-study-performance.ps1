[CmdletBinding()]
param([ValidateRange(15,120)][int]$DurationSeconds=30,[ValidateRange(1,20)][int]$Clients=10)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'unit-study-common.ps1')
$originalProject=$script:LedgerSyncComposeProject
$root=$studyRoot
$benchProject="ledgersync-acceptance-$([DateTime]::UtcNow.ToString('yyyyMMddHHmmss'))-$([Guid]::NewGuid().ToString('N').Substring(0,8))"
$benchStatePath=Join-Path $root 'data/unit-study/performance-active.json'
$evidenceDirectory=Join-Path $root 'docs/release-evidence/unit-3-unit-4/performance'
New-Item -ItemType Directory -Force -Path $evidenceDirectory | Out-Null
@{project=$benchProject;created_at=[DateTime]::UtcNow.ToString('o');purpose='disposable performance matrix'} | ConvertTo-Json | Set-Content -LiteralPath $benchStatePath
$k6Image='grafana/k6@sha256:1f40432b1cbe7234e977f96c362c9bc550a2d2b583d014dd8669fe40d3e9e755'
$machine=(& docker info --format '{{json .}}' | ConvertFrom-Json)
$environment=@{engine=$machine.ServerVersion;cpu_count=$machine.NCPU;memory_bytes=$machine.MemTotal;per_api_cpu=1;per_worker_cpu=1;per_api_memory_mib=256;per_worker_memory_mib=256;database_pool_per_instance=4;background_containers=@(& docker ps --format '{{.Names}}')}
$manifest=@(& git -C $root ls-files --cached --others --exclude-standard) | Sort-Object -Unique | ForEach-Object {
    $path=Join-Path $root $_
    if((Test-Path -LiteralPath $path -PathType Leaf) -and $_ -notmatch '^docs/release-evidence/unit-3-unit-4/') {@{path=$_;sha256=(Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()}}
}
$sourceJSON=$manifest | ConvertTo-Json -Depth 3 -Compress
$sourceFingerprint=[Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($sourceJSON))).ToLowerInvariant()
$imageIDs=@{}
foreach($role in @('api','worker','web','migrate','proxy')) {
    $imageIDs[$role]=[string](& docker image inspect --format '{{.Id}}' "$originalProject-$role")
    if($LASTEXITCODE -ne 0) {throw 'Study images must be built before the performance matrix.'}
    & docker tag "$originalProject-$role" "$benchProject-$role"
    if($LASTEXITCODE -ne 0) {throw 'Could not tag the fixed study images.'}
}
function Get-BenchmarkData {
    Invoke-LedgerSyncAcceptanceSQLJSON -SQL "SELECT jsonb_build_object('accounts',(SELECT count(*) FROM accounts),'transfers',(SELECT count(*) FROM transfers),'journals',(SELECT count(*) FROM journal_transactions),'postings',(SELECT count(*) FROM ledger_postings),'opening_total',(SELECT sum(opening_ledger_minor)::text FROM account_opening_balances),'pending',(SELECT count(*) FROM outbox_events WHERE published_at IS NULL AND dead_at IS NULL),'dead',(SELECT count(*) FROM outbox_events WHERE dead_at IS NOT NULL),'published',(SELECT count(*) FROM outbox_events WHERE published_at IS NOT NULL),'database_bytes',pg_database_size('ledgersync'),'oldest_pending_seconds',(SELECT COALESCE(EXTRACT(EPOCH FROM clock_timestamp()-min(created_at)),0) FROM outbox_events WHERE published_at IS NULL AND dead_at IS NULL),'unbalanced',(SELECT count(*) FROM (SELECT journal_transaction_id FROM ledger_postings GROUP BY journal_transaction_id HAVING count(*)<>2 OR sum(CASE WHEN direction='debit' THEN -amount_minor ELSE amount_minor END)<>0) invalid),'duplicate_journals',(SELECT count(*) FROM (SELECT transfer_id FROM journal_transactions GROUP BY transfer_id HAVING count(*)<>1) invalid));"
}
function Invoke-BenchmarkLoad {
    param([string]$Name,[string]$Shape,[int]$Seconds,[switch]$Sample)
    $pairs='10000000-0000-4000-8000-000000000001>10000000-0000-4000-8000-000000000004'
    if($Shape -eq 'independent') {$pairs+=';10000000-0000-4000-8000-000000000002>10000000-0000-4000-8000-000000000003'}
    $container="$benchProject-load"
    $arguments=@('run','--rm','--name',$container,'--network','host','-v',"$($evidenceDirectory.Replace('\','/')):/results",'-v',"$($root.Replace('\','/'))/tests/performance/k6:/workload:ro",$k6Image,'run','--quiet',"--summary-export=/results/$Name.json",'--summary-trend-stats=avg,min,med,p(50),p(95),p(99),max','-e','LEDGERSYNC_PERF_BFF_URL=http://127.0.0.1:3102','-e','LEDGERSYNC_PERF_PUBLIC_ORIGIN=http://127.0.0.1:3102','-e','LEDGERSYNC_PERF_LOCAL_SIGN_IN=true','-e',"LEDGERSYNC_PERF_WORKLOAD_SHAPE=$Shape",'-e',"LEDGERSYNC_PERF_ACCOUNT_PAIRS=$pairs",'-e',"LEDGERSYNC_PERF_TPS=$Clients",'-e',"LEDGERSYNC_PERF_DURATION=$($Seconds)s",'/workload/transfers.js')
    $configuration=@{arguments=$arguments;log=(Join-Path $evidenceDirectory "$Name.log")}
    $job=Start-Job -ArgumentList $configuration -ScriptBlock {param($configuration) & docker @($configuration.arguments) *> $configuration.log;return $LASTEXITCODE}
    $samples=@();$deadline=[DateTime]::UtcNow.AddSeconds($Seconds+120)
    try {
        while($job.State -eq 'Running') {
            if([DateTime]::UtcNow -gt $deadline) {throw 'The bounded load deadline expired.'}
            if($Sample) {
                $containers=@(& docker ps --filter "label=com.docker.compose.project=$benchProject" --format '{{.Names}}')
                $stats=@(& docker stats --no-stream --format '{{json .}}' @containers | ForEach-Object {$_ | ConvertFrom-Json})
                $samples+=@{at=[DateTime]::UtcNow.ToString('o');containers=$stats;database=(Get-BenchmarkData)}
            }
            Start-Sleep -Seconds 3
        }
        $exitCode=@(Receive-Job -Job $job -Wait)[-1]
        if(-not(Test-Path -LiteralPath (Join-Path $evidenceDirectory "$Name.json"))) {throw "Load summary missing. See $Name.log."}
        return @{exit_code=[int]$exitCode;summary=(Get-Content -LiteralPath (Join-Path $evidenceDirectory "$Name.json") -Raw | ConvertFrom-Json);samples=$samples}
    } finally {
        if($job.State -eq 'Running') {& docker stop --time 1 $container | Out-Null;Stop-Job -Job $job}
        Remove-Job -Job $job -Force
    }
}
$results=@();$seedSignature=$null
try {
    Invoke-LedgerSyncCompose -ComposeArguments @('--profile','scheduler','stop','reconciliation-scheduler','web','web-peer','study-proxy','api','outbox-worker','postgres','redis')
    $env:LEDGERSYNC_STUDY_ACTIVE_PATH=$benchStatePath
    . (Join-Path $PSScriptRoot 'unit-study-common.ps1')
    Initialize-LedgerSyncLocalSecrets
    foreach($shape in @('independent','hot')) {
        foreach($configuration in @(@{api=1;worker=1},@{api=2;worker=1},@{api=1;worker=2},@{api=2;worker=2})) {
            $name="$shape-api$($configuration.api)-worker$($configuration.worker)"
            Write-Host "Measuring $name with fresh synthetic data."
            # Exact project identity is checked by unit-study-common before this deletion.
            Invoke-LedgerSyncCompose -ComposeArguments @('--profile','scheduler','down','--volumes','--remove-orphans')
            & (Join-Path $PSScriptRoot 'start-unit-study.ps1') -SkipBuild -APIReplicas $configuration.api -WorkerReplicas $configuration.worker
            $seed=Get-BenchmarkData
            $signature="$($seed.accounts)/$($seed.transfers)/$($seed.journals)/$($seed.postings)/$($seed.opening_total)"
            if($null -eq $seedSignature) {$seedSignature=$signature} elseif($seedSignature -cne $signature) {throw 'Seed data changed between matrix runs.'}
            $warm=Invoke-BenchmarkLoad -Name "$name-warmup" -Shape $shape -Seconds 5
            if($warm.summary.metrics.ledgersync_unexpected_outcomes.count -gt 0) {throw "Warm-up failed. See $name-warmup.log."}
            $warmDeadline=[DateTime]::UtcNow.AddSeconds(30)
            do {$warmQueue=Get-BenchmarkData;if($warmQueue.pending -eq 0){break};Start-Sleep -Milliseconds 500} while([DateTime]::UtcNow -lt $warmDeadline)
            if($warmQueue.pending -ne 0) {throw 'Warm-up delivery did not drain.'}
            $before=Get-BenchmarkData
            $load=Invoke-BenchmarkLoad -Name $name -Shape $shape -Seconds $DurationSeconds -Sample
            $deadline=[DateTime]::UtcNow.AddSeconds(30)
            do {$after=Get-BenchmarkData;if($after.pending -eq 0){break};Start-Sleep -Milliseconds 500} while([DateTime]::UtcNow -lt $deadline)
            $reconciliation=Invoke-LedgerSyncAcceptanceReconciliation -TenantID '00000000-0000-4000-8000-000000000001'
            $correct=$after.unbalanced -eq 0 -and $after.duplicate_journals -eq 0 -and $after.dead -eq 0 -and $after.pending -eq 0 -and $load.summary.metrics.ledgersync_unexpected_outcomes.count -eq 0 -and ($after.journals-$before.journals) -eq $load.summary.metrics.ledgersync_posted_commands.count
            $entry=@{name=$name;api_replicas=$configuration.api;worker_replicas=$configuration.worker;seed=$seed;before=$before;after=$after;load=$load;reconciliation=$reconciliation;correctness_passed=$correct;captured_at=[DateTime]::UtcNow.ToString('o')}
            $results+=$entry
            $entry | ConvertTo-Json -Depth 30 | Set-Content -LiteralPath (Join-Path $evidenceDirectory "$name-report.json")
            if(-not $correct) {throw 'Performance acceptance stopped on a correctness failure.'}
        }
    }
} finally {
    if($script:LedgerSyncComposeProject -ceq $benchProject) {Invoke-LedgerSyncCompose -ComposeArguments @('--profile','scheduler','down','--volumes','--remove-orphans')}
    $env:LEDGERSYNC_STUDY_ACTIVE_PATH=$null
    & (Join-Path $PSScriptRoot 'start-unit-study.ps1') -SkipBuild -APIReplicas 2 -WorkerReplicas 2 -IncludePeer -WithScheduler
    @{project=$benchProject;original_project=$originalProject;duration_seconds=$DurationSeconds;warmup_seconds=5;paced_clients=$Clients;image_ids=$imageIDs;environment=$environment;source_revision=(& git -C $root rev-parse HEAD).Trim();source_fingerprint=$sourceFingerprint;seed_signature=$seedSignature;results=$results;limits='One Docker host. Per-service limits stay fixed; total limits increase with replicas. Ten paced clients are not a guaranteed arrival rate. Short runs do not establish production capacity.'} | ConvertTo-Json -Depth 32 | Set-Content -LiteralPath (Join-Path $evidenceDirectory 'matrix.json')
}
