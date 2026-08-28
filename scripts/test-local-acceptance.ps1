[CmdletBinding()]
param(
    [switch]$IncludeCapacity
)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$normalProject = 'compose'
$acceptanceProject = "ledgersync-acceptance-$((Get-Date).ToUniversalTime().ToString('yyyyMMddHHmmss'))-$([Guid]::NewGuid().ToString('N').Substring(0,8))"
$acceptanceRoot = [IO.Path]::GetFullPath((Join-Path $repositoryRoot 'data\local-acceptance'))
$acceptanceState = [IO.Path]::GetFullPath((Join-Path $acceptanceRoot $acceptanceProject))
$backupRoot = [IO.Path]::GetFullPath((Join-Path $acceptanceState 'backups'))
$sourceAccountID = '10000000-0000-4000-8000-000000000001'
$tenantID = '00000000-0000-4000-8000-000000000001'
$normalWasStopped = $false
$acceptanceCreated = $false
$acceptancePassed = $false
$capacityPassed = $false
$browserPassed = $false
$cleanupPassed = $false
$failureMessage = ''
$commit = ''
$migrationVersion = ''
$startedAt = [DateTimeOffset]::UtcNow

. (Join-Path $PSScriptRoot 'local-runtime-common.ps1')
. (Join-Path $PSScriptRoot 'local-acceptance-common.ps1')
. (Join-Path $PSScriptRoot 'local-backup-common.ps1')

function Remove-LedgerSyncAcceptanceState {
    Assert-LedgerSyncAcceptanceProjectIdentity -Project $acceptanceProject -StateRoot $acceptanceRoot -StatePath $acceptanceState
    if (Test-Path -LiteralPath $acceptanceState) { Remove-Item -LiteralPath $acceptanceState -Recurse -Force }
}

function Restart-LedgerSyncAcceptanceService {
    param([Parameter(Mandatory = $true)][ValidateSet('redis','outbox-worker','api','web','postgres')][string]$Service)
    Invoke-LedgerSyncCompose -ComposeArguments @('restart',$Service) | Out-Null
    if ($Service -eq 'postgres') {
        Invoke-LedgerSyncCompose -ComposeArguments @('up','-d','--wait','postgres') | Out-Null
        Invoke-LedgerSyncCompose -ComposeArguments @('run','--rm','migrate') | Out-Null
        Invoke-LedgerSyncCompose -ComposeArguments @('up','-d','--wait','api','outbox-worker','web') | Out-Null
    } else {
        Invoke-LedgerSyncCompose -ComposeArguments @('up','-d','--wait',$Service) | Out-Null
    }
    Assert-LedgerSyncLongRunningServicesHealthy
    Invoke-LedgerSyncWebSmoke
}

try {
    $branch = (& git -C $repositoryRoot branch --show-current).Trim()
    $commit = (& git -C $repositoryRoot rev-parse HEAD).Trim()
    Assert-LedgerSyncAcceptance ($LASTEXITCODE -eq 0 -and $branch -ceq 'main' -and $commit -cmatch '^[0-9a-f]{40}$') "Acceptance requires the committed main branch."
    $trackedChanges = @(& git -C $repositoryRoot status --porcelain --untracked-files=no)
    Assert-LedgerSyncAcceptance ($LASTEXITCODE -eq 0 -and $trackedChanges.Count -eq 0) "Acceptance requires a clean tracked tree after harness preparation is committed."
    Assert-LedgerSyncAcceptanceProjectIdentity -Project $acceptanceProject -StateRoot $acceptanceRoot -StatePath $acceptanceState
    Assert-LedgerSyncDockerAvailable
    $dockerResources = (& docker info --format '{{.NCPU}}|{{.MemTotal}}').Trim().Split('|')
    Assert-LedgerSyncAcceptance ($LASTEXITCODE -eq 0 -and $dockerResources.Count -eq 2 -and [int]$dockerResources[0] -ge 2 -and [int64]$dockerResources[1] -ge 4GB) "Acceptance requires at least two Docker CPUs and 4 GiB of memory."
    Assert-LedgerSyncAcceptance ($script:LedgerSyncComposeProject -ceq $normalProject) "Acceptance preflight did not resolve the normal Compose project."
    Test-LedgerSyncPortAvailableOrOwned
    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy
    Invoke-LedgerSyncWebSmoke
    $normalBefore = Get-LedgerSyncAcceptanceNormalState
    Assert-LedgerSyncAcceptance ([int64]$normalBefore.Operational.outbox_pending -eq 0 -and [int64]$normalBefore.Operational.outbox_dead -eq 0 -and [int64]$normalBefore.Operational.reconciliation_mismatches -eq 0 -and [string]$normalBefore.Operational.reconciliation_status -in @('matched','completed','passed')) "Normal preflight state was not clean."
    $containerCollision = @(& docker ps -a --filter "label=com.docker.compose.project=$acceptanceProject" --format '{{.ID}}')
    $volumeCollision = @(& docker volume ls --filter "label=com.docker.compose.project=$acceptanceProject" --format '{{.Name}}')
    $networkCollision = @(& docker network ls --filter "label=com.docker.compose.project=$acceptanceProject" --format '{{.Name}}')
    Assert-LedgerSyncAcceptance ($containerCollision.Count -eq 0 -and $volumeCollision.Count -eq 0 -and $networkCollision.Count -eq 0 -and -not (Test-Path -LiteralPath $acceptanceState)) "Acceptance refused an existing exact project resource or state path."

    & pwsh -NoProfile -File (Join-Path $PSScriptRoot 'stop-local.ps1') | Out-Null
    Assert-LedgerSyncAcceptance ($LASTEXITCODE -eq 0) "Normal project could not be stopped without deleting data."
    $normalWasStopped = $true
    $env:LEDGERSYNC_LOCAL_COMPOSE_PROJECT = $acceptanceProject
    $env:LEDGERSYNC_LOCAL_STATE_DIRECTORY = $acceptanceState
    . (Join-Path $PSScriptRoot 'local-runtime-common.ps1')
    . (Join-Path $PSScriptRoot 'local-acceptance-common.ps1')
    New-Item -ItemType Directory -Path $acceptanceRoot -Force | Out-Null
    $acceptanceCreated = $true
    & pwsh -NoProfile -File (Join-Path $PSScriptRoot 'start-local.ps1') | Out-Null
    Assert-LedgerSyncAcceptance ($LASTEXITCODE -eq 0) "Fresh acceptance stack did not start."
    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy
    Invoke-LedgerSyncWebSmoke
    Assert-LedgerSyncAcceptanceRuntimeHardening -Project $acceptanceProject -RuntimeEnvironmentFile $script:LedgerSyncRuntimeEnvironmentFile

    $session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
    $sessionPayload = (Invoke-LedgerSyncAcceptanceJSON -Session $session -Method GET -Path '/api/session').Payload
    Assert-LedgerSyncAcceptance ([string]$sessionPayload.environment -ceq 'demo' -and [string]$sessionPayload.subject_id -ceq 'demo-operator' -and [string]$sessionPayload.tenant_id -ceq $tenantID) "Direct demo session identity drifted."
    $requiredScopes = @('accounts:read','accounts:write','transactions:read','transfers:read','transfers:write','reconciliation:read','reconciliation:write','local:read','events:read','explainability:read','recovery:read','exports:read')
    Assert-LedgerSyncAcceptance (@($requiredScopes | Where-Object { @($sessionPayload.scopes) -notcontains $_ }).Count -eq 0) "Direct demo session omitted a required acceptance scope."
    $csrf = [string]$sessionPayload.csrf_token
    Assert-LedgerSyncAcceptance ($csrf.Length -ge 32) "Direct demo session omitted its CSRF value."
    $overview = Invoke-WebRequest -UseBasicParsing -WebSession $session -TimeoutSec 15 -Uri $script:LedgerSyncWebUrl
    Assert-LedgerSyncAcceptance ([int]$overview.StatusCode -eq 200 -and $overview.Content -match 'Exact, explainable internal ledger transfers and balances') "Acceptance overview omitted the trust promise."
    Test-LedgerSyncAcceptanceOrientation -Session $session | Out-Null

    $accounts = (Invoke-LedgerSyncAcceptanceJSON -Session $session -Method GET -Path '/api/me/accounts?limit=10').Payload
    Assert-LedgerSyncAcceptance (@($accounts.accounts).Count -eq 6 -and @($accounts.accounts | Where-Object account_id -eq $sourceAccountID).Count -eq 1) "Fresh baseline account directory drifted."
    $baselineSource = (Invoke-LedgerSyncAcceptanceJSON -Session $session -Method GET -Path "/api/accounts/$sourceAccountID").Payload
    Assert-LedgerSyncAcceptance ([string]$baselineSource.currency -ceq 'INR' -and [string]$baselineSource.status -ceq 'active' -and [string]$baselineSource.account_version -ceq '1') "Baseline source account truth drifted."
    $baselineHistory = (Invoke-LedgerSyncAcceptanceJSON -Session $session -Method GET -Path "/api/accounts/$sourceAccountID/transactions?limit=20").Payload
    Assert-LedgerSyncAcceptance (@($baselineHistory.transactions).Count -eq 2) "Baseline source history did not preserve both seeded postings."

    $runLabel = [Guid]::NewGuid().ToString('N').Substring(0,12)
    $created = Test-LedgerSyncAcceptanceAccountCreationBoundary -Session $session -CSRFToken $csrf -RunLabel $runLabel
    $accountID = [string]$created.AccountID
    $fundingKey = "fund-lost-$runLabel"
    $funding = Test-LedgerSyncAcceptanceLostTransferBoundary -Session $session -CSRFToken $csrf -SourceAccountID $sourceAccountID -DestinationAccountID $accountID -AmountMinor '300' -IdempotencyKey $fundingKey
    Test-LedgerSyncAcceptanceTransferDetail -Session $session -TransferID $funding.TransferID -AmountMinor '300'
    $funded = (Invoke-LedgerSyncAcceptanceJSON -Session $session -Method GET -Path "/api/accounts/$accountID").Payload
    Assert-LedgerSyncAcceptance ([string]$funded.available_minor -ceq '300' -and [string]$funded.ledger_minor -ceq '300' -and [string]$funded.status -ceq 'active') "Created account did not expose its exact funded state immediately."

    $freeze = Invoke-LedgerSyncAcceptanceAccountStatus -Session $session -CSRFToken $csrf -AccountID $accountID -ExpectedVersion ([string]$funded.account_version) -TargetStatus frozen -Reason 'Acceptance freeze proof' -IdempotencyKey "freeze-$runLabel"
    $frozenBalance = Get-LedgerSyncAcceptanceBalance -Session $session -AccountID $accountID
    $frozenTransferKey = "frozen-transfer-$runLabel"
    Invoke-LedgerSyncAcceptanceTransferOutcome -Session $session -CSRFToken $csrf -SourceAccountID $accountID -DestinationAccountID $sourceAccountID -AmountMinor '100' -IdempotencyKey $frozenTransferKey -ExpectedStatus 409 -ExpectedErrorCode 'account_inactive' | Out-Null
    # Rejected transfer attempts are revalidated rather than persisted as a
    # completed idempotent movement. A same-key retry must deny again and must
    # still leave PostgreSQL money unchanged.
    Invoke-LedgerSyncAcceptanceTransferOutcome -Session $session -CSRFToken $csrf -SourceAccountID $accountID -DestinationAccountID $sourceAccountID -AmountMinor '100' -IdempotencyKey $frozenTransferKey -ExpectedStatus 409 -ExpectedErrorCode 'account_inactive' | Out-Null
    $frozenAfter = Get-LedgerSyncAcceptanceBalance -Session $session -AccountID $accountID
    Assert-LedgerSyncAcceptance ([string]$frozenAfter.available_minor -ceq [string]$frozenBalance.available_minor -and [string]$frozenAfter.ledger_minor -ceq [string]$frozenBalance.ledger_minor) "Frozen transfer denial changed money."
    $reactivated = Invoke-LedgerSyncAcceptanceAccountStatus -Session $session -CSRFToken $csrf -AccountID $accountID -ExpectedVersion ([string]$freeze.Payload.account_version) -TargetStatus active -Reason 'Acceptance reactivation proof' -IdempotencyKey "reactivate-$runLabel"

    $eligible = Invoke-LedgerSyncAcceptanceTransferOutcome -Session $session -CSRFToken $csrf -SourceAccountID $accountID -DestinationAccountID $sourceAccountID -AmountMinor '100' -IdempotencyKey "eligible-$runLabel"
    Test-LedgerSyncAcceptanceTransferDetail -Session $session -TransferID ([string]$eligible.Response.Payload.transfer_id) -AmountMinor '100'
    $nonzero = (Invoke-LedgerSyncAcceptanceJSON -Session $session -Method GET -Path "/api/accounts/$accountID").Payload
    Assert-LedgerSyncAcceptance ([string]$nonzero.available_minor -ceq '200' -and [string]$nonzero.ledger_minor -ceq '200') "Eligible transfer did not leave the exact nonzero balance."
    $nonzeroCloseKey = "close-nonzero-$runLabel"
    Invoke-LedgerSyncAcceptanceAccountStatus -Session $session -CSRFToken $csrf -AccountID $accountID -ExpectedVersion ([string]$nonzero.account_version) -TargetStatus closed -Reason 'Acceptance nonzero close proof' -IdempotencyKey $nonzeroCloseKey -ExpectedStatus 422 -ExpectedErrorCode 'account_not_zero' | Out-Null
    Invoke-LedgerSyncAcceptanceAccountStatus -Session $session -CSRFToken $csrf -AccountID $accountID -ExpectedVersion ([string]$nonzero.account_version) -TargetStatus closed -Reason 'Acceptance nonzero close proof' -IdempotencyKey $nonzeroCloseKey -ExpectedStatus 422 -ExpectedErrorCode 'account_not_zero' | Out-Null
    $nonzeroAfterRetry = (Invoke-LedgerSyncAcceptanceJSON -Session $session -Method GET -Path "/api/accounts/$accountID").Payload
    Assert-LedgerSyncAcceptance ([string]$nonzeroAfterRetry.status -ceq 'active' -and [string]$nonzeroAfterRetry.account_version -ceq [string]$nonzero.account_version -and [string]$nonzeroAfterRetry.available_minor -ceq [string]$nonzero.available_minor -and [string]$nonzeroAfterRetry.ledger_minor -ceq [string]$nonzero.ledger_minor) "Repeated nonzero close denial changed account or financial state."

    $drain = Invoke-LedgerSyncAcceptanceTransferOutcome -Session $session -CSRFToken $csrf -SourceAccountID $accountID -DestinationAccountID $sourceAccountID -AmountMinor '200' -IdempotencyKey "drain-$runLabel"
    Test-LedgerSyncAcceptanceTransferDetail -Session $session -TransferID ([string]$drain.Response.Payload.transfer_id) -AmountMinor '200'
    $zero = (Invoke-LedgerSyncAcceptanceJSON -Session $session -Method GET -Path "/api/accounts/$accountID").Payload
    Assert-LedgerSyncAcceptance ([string]$zero.available_minor -ceq '0' -and [string]$zero.ledger_minor -ceq '0') "Drain did not produce exact zero balances."
    Invoke-LedgerSyncAcceptanceAccountStatus -Session $session -CSRFToken $csrf -AccountID $accountID -ExpectedVersion ([string]$zero.account_version) -TargetStatus closed -Reason 'Acceptance terminal close proof' -IdempotencyKey "close-zero-$runLabel" | Out-Null
    $closedDetail = (Invoke-LedgerSyncAcceptanceJSON -Session $session -Method GET -Path "/api/accounts/$accountID").Payload
    Assert-LedgerSyncAcceptance ([string]$closedDetail.status -ceq 'closed' -and [string]$closedDetail.account_version -ceq '4' -and [string]$closedDetail.available_minor -ceq '0' -and [string]$closedDetail.ledger_minor -ceq '0') "Terminal close did not preserve version four and exact zero truth."
    $auditReasons = @($closedDetail.audit_context | ForEach-Object {
        if ($_.PSObject.Properties.Name -contains 'reason') { [string]$_.reason }
    })
    Assert-LedgerSyncAcceptance ($auditReasons -contains 'Acceptance freeze proof' -and $auditReasons -contains 'Acceptance reactivation proof' -and $auditReasons -contains 'Acceptance nonzero close proof' -and $auditReasons -contains 'Acceptance terminal close proof') "Account detail omitted sanitized lifecycle or denial reasons."
    Invoke-LedgerSyncAcceptanceTransferOutcome -Session $session -CSRFToken $csrf -SourceAccountID $accountID -DestinationAccountID $sourceAccountID -AmountMinor '100' -IdempotencyKey "closed-transfer-$runLabel" -ExpectedStatus 409 -ExpectedErrorCode 'account_inactive' | Out-Null
    $finalHistory = (Invoke-LedgerSyncAcceptanceJSON -Session $session -Method GET -Path "/api/accounts/$accountID/transactions?limit=100").Payload
    foreach ($transferID in @([string]$funding.TransferID,[string]$eligible.Response.Payload.transfer_id,[string]$drain.Response.Payload.transfer_id)) {
        Assert-LedgerSyncAcceptance (@($finalHistory.transactions | Where-Object transfer_id -eq $transferID).Count -eq 1) "Closed-account history omitted or duplicated a committed transfer."
    }

    $runID = Test-LedgerSyncAcceptanceReconciliationBoundary -Session $session -CSRFToken $csrf -IdempotencyKey "reconcile-$runLabel"
    Test-LedgerSyncAcceptanceOperationalEvidence -Session $session -TransferID ([string]$funding.TransferID)
    Test-LedgerSyncAcceptanceExports -Session $session -AccountID $accountID -TransferID ([string]$funding.TransferID) -RunID $runID -AmountMinor '300'
    $recovery = (Invoke-LedgerSyncAcceptanceJSON -Session $session -Method GET -Path '/api/recovery/manifests').Payload
    Assert-LedgerSyncAcceptance ([string]$recovery.format_version -ceq 'ledgersync-recovery-evidence-index/v1' -and [int]$recovery.retention.valid_backup_count -ge 0) "Recovery view did not expose a valid sanitized v1 index."
    $orientationAfter = Test-LedgerSyncAcceptanceOrientation -Session $session
    foreach ($step in @('create_account','fund_account','inspect_transfer','run_reconciliation')) {
        Assert-LedgerSyncAcceptance ([string](@($orientationAfter.steps | Where-Object id -eq $step)[0].state) -in @('completed','evidence_available')) "Orientation did not converge on durable journey evidence."
    }

    $env:LEDGERSYNC_SYSTEM_ALLOW_LEDGER_MUTATION = 'true'
    $env:LEDGERSYNC_SYSTEM_ISOLATED_PROJECT = 'true'
    $env:LEDGERSYNC_SYSTEM_WEB_URL = $script:LedgerSyncWebUrl
    $env:LEDGERSYNC_SYSTEM_COMPOSE_PROJECT = $acceptanceProject
    $env:LEDGERSYNC_SYSTEM_SEEDED_SOURCE_ACCOUNT_ID = $sourceAccountID
    $env:LEDGERSYNC_SYSTEM_RUN_ID = "phase10-$([Guid]::NewGuid().ToString('N').Substring(0,12))"
    try {
        Push-Location (Join-Path $repositoryRoot 'web')
        $nativeErrorPreference = $PSNativeCommandUseErrorActionPreference
        $PSNativeCommandUseErrorActionPreference = $false
        try {
            $browserOutput = @(& npm run test:e2e:real-stack 2>&1)
            $browserExitCode = $LASTEXITCODE
        }
        finally {
            $PSNativeCommandUseErrorActionPreference = $nativeErrorPreference
        }
        $browserTail = @(
            $browserOutput |
                Select-Object -Last 12 |
                Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) } |
                ForEach-Object { ConvertTo-LedgerSyncRedactedLogLine -Line ([string]$_) }
        ) -join ' | '
        Assert-LedgerSyncAcceptance ($browserExitCode -eq 0) "The isolated real-browser product journey failed: $browserTail"
        $browserPassed = $true
    }
    finally {
        Pop-Location
        foreach ($name in @('LEDGERSYNC_SYSTEM_ALLOW_LEDGER_MUTATION','LEDGERSYNC_SYSTEM_ISOLATED_PROJECT','LEDGERSYNC_SYSTEM_WEB_URL','LEDGERSYNC_SYSTEM_COMPOSE_PROJECT','LEDGERSYNC_SYSTEM_SEEDED_SOURCE_ACCOUNT_ID','LEDGERSYNC_SYSTEM_RUN_ID')) {
            Remove-Item "Env:$name" -ErrorAction SilentlyContinue
        }
    }

    Restart-LedgerSyncAcceptanceService -Service redis
    Invoke-LedgerSyncCompose -ComposeArguments @('exec','-T','redis','redis-cli','FLUSHDB') -CaptureOutput | Out-Null
    $redisBalance = Get-LedgerSyncAcceptanceBalance -Session $session -AccountID $accountID
    Assert-LedgerSyncAcceptance ([string]$redisBalance.available_minor -ceq '0') "Redis restart changed authoritative money."
    Invoke-LedgerSyncAcceptanceReconciliation -TenantID $tenantID | Out-Null
    $redisKeys = @(Invoke-LedgerSyncCompose -ComposeArguments @('exec','-T','redis','redis-cli','DBSIZE') -CaptureOutput | Where-Object { [string]$_ -cmatch '^[0-9]+$' } | Select-Object -Last 1)
    Assert-LedgerSyncAcceptance ($redisKeys.Count -eq 1 -and [int64]$redisKeys[0] -gt 0) "Redis rebuild produced no bounded cache evidence."
    Invoke-LedgerSyncCompose -ComposeArguments @('stop','outbox-worker') | Out-Null
    $backlogAccount = Test-LedgerSyncAcceptanceAccountCreationBoundary -Session $session -CSRFToken $csrf -RunLabel ([Guid]::NewGuid().ToString('N').Substring(0,12))
    $expiredLease = Invoke-LedgerSyncAcceptanceSQLJSON -SQL "WITH claimed AS (UPDATE outbox_events SET claim_owner='acceptance-expired-lease',claimed_until=now()-interval '1 minute' WHERE account_id='$([string]$backlogAccount.AccountID)'::uuid AND published_at IS NULL AND dead_at IS NULL RETURNING id) SELECT json_build_object('claimed',(SELECT count(*) FROM claimed));"
    Assert-LedgerSyncAcceptance ([int]$expiredLease.claimed -eq 1) "Acceptance could not establish one isolated expired worker lease."
    $backlog = Get-LedgerSyncOperationalSummary
    Assert-LedgerSyncAcceptance ([int64]$backlog.outbox_pending -ge 1 -and [int64]$backlog.outbox_dead -eq 0) "Stopped worker did not expose truthful pending outbox state."
    Restart-LedgerSyncAcceptanceService -Service outbox-worker
    Wait-LedgerSyncAcceptanceOutbox | Out-Null
    $leaseRecovery = Invoke-LedgerSyncAcceptanceSQLJSON -SQL "SELECT json_build_object('published',(SELECT count(*) FROM outbox_events WHERE account_id='$([string]$backlogAccount.AccountID)'::uuid AND published_at IS NOT NULL AND dead_at IS NULL AND claim_owner IS NULL));"
    Assert-LedgerSyncAcceptance ([int]$leaseRecovery.published -eq 1) "Worker restart did not recover the isolated expired lease exactly once."
    Restart-LedgerSyncAcceptanceService -Service api
    $apiSession = (Invoke-LedgerSyncAcceptanceJSON -Session $session -Method GET -Path '/api/session').Payload
    Assert-LedgerSyncAcceptance ([string]$apiSession.tenant_id -ceq $tenantID) "API restart changed the active session boundary."
    $fundingReplay = Invoke-LedgerSyncAcceptanceTransferOutcome -Session $session -CSRFToken $csrf -SourceAccountID $sourceAccountID -DestinationAccountID $accountID -AmountMinor '300' -IdempotencyKey $fundingKey -ExpectReplay
    Assert-LedgerSyncAcceptance ([string]$fundingReplay.Response.Payload.transfer_id -ceq [string]$funding.TransferID) "API restart lost transfer idempotency evidence."
    Restart-LedgerSyncAcceptanceService -Service web
    $webSession = (Invoke-LedgerSyncAcceptanceJSON -Session $session -Method GET -Path '/api/session').Payload
    Assert-LedgerSyncAcceptance ([string]$webSession.tenant_id -ceq $tenantID) "Web restart changed the active session boundary."
    Restart-LedgerSyncAcceptanceService -Service postgres
    $postgresDetail = (Invoke-LedgerSyncAcceptanceJSON -Session $session -Method GET -Path "/api/accounts/$accountID").Payload
    Assert-LedgerSyncAcceptance ([string]$postgresDetail.status -ceq 'closed' -and [string]$postgresDetail.available_minor -ceq '0') "PostgreSQL restart lost closed financial state."
    Test-LedgerSyncAcceptanceReconciliationBoundary -Session $session -CSRFToken $csrf -IdempotencyKey "reconcile-final-$runLabel" | Out-Null
    $finalSummary = Get-LedgerSyncOperationalSummary
    Assert-LedgerSyncAcceptance ([int64]$finalSummary.outbox_pending -eq 0 -and [int64]$finalSummary.outbox_dead -eq 0 -and [int64]$finalSummary.reconciliation_mismatches -eq 0) "Final isolated operational state was not clean."
    $migrationVersion = [string]$finalSummary.migration_version
    $durableEvidence = Invoke-LedgerSyncAcceptanceSQLJSON -SQL "SELECT json_build_object('account',(SELECT count(*) FROM accounts WHERE id='$accountID'::uuid AND status='closed'),'projection',(SELECT count(*) FROM account_balance_projections WHERE account_id='$accountID'::uuid AND available_minor=0 AND ledger_minor=0),'posted_transfers',(SELECT count(*) FROM transfers WHERE (debit_account_id='$accountID'::uuid OR credit_account_id='$accountID'::uuid) AND status='posted'),'journals',(SELECT count(*) FROM journal_transactions j JOIN transfers t ON t.id=j.transfer_id WHERE t.debit_account_id='$accountID'::uuid OR t.credit_account_id='$accountID'::uuid),'postings',(SELECT count(*) FROM ledger_postings p JOIN journal_transactions j ON j.id=p.journal_transaction_id JOIN transfers t ON t.id=j.transfer_id WHERE t.debit_account_id='$accountID'::uuid OR t.credit_account_id='$accountID'::uuid),'audit',(SELECT count(*) FROM audit_events WHERE target_id='$accountID'),'outbox',(SELECT count(*) FROM outbox_events WHERE account_id='$accountID'::uuid),'matched_reconciliations',(SELECT count(*) FROM reconciliation_runs WHERE status='matched' AND mismatch_count=0));"
    Assert-LedgerSyncAcceptance ([int]$durableEvidence.account -eq 1 -and [int]$durableEvidence.projection -eq 1 -and [int]$durableEvidence.posted_transfers -eq 3 -and [int]$durableEvidence.journals -eq 3 -and [int]$durableEvidence.postings -eq 6 -and [int]$durableEvidence.audit -ge 5 -and [int]$durableEvidence.outbox -ge 7 -and [int]$durableEvidence.matched_reconciliations -ge 1) "Pre-backup durable lifecycle/ledger/audit/outbox/reconciliation evidence was incomplete."

    if ($IncludeCapacity) {
        $capacityEvidencePath = Join-Path $repositoryRoot ".tmp/capacity-phase1/$acceptanceProject.json"
        $capacityOutput = @(& pwsh -NoProfile -File (Join-Path $PSScriptRoot 'run-capacity-qualification.ps1') -WorkloadShape mixed -TransactionsPerSecond 25 -Duration 5m -ComposeProject $acceptanceProject -OutputPath $capacityEvidencePath)
        Assert-LedgerSyncAcceptance ($LASTEXITCODE -eq 0) "The isolated five-minute capacity qualification failed."
        $capacityLine = @($capacityOutput | Where-Object { ([string]$_).TrimStart().StartsWith('{') } | Select-Object -Last 1)
        Assert-LedgerSyncAcceptance ($capacityLine.Count -eq 1) "Capacity qualification returned no bounded JSON decision."
        $capacityDecision = [string](($capacityLine[0] | ConvertFrom-Json).decision)
        Assert-LedgerSyncAcceptance ($capacityDecision -ceq 'pass') "Capacity qualification did not return a pass decision."
        $capacityPassed = $true
    }

    $backupOutput = @(& pwsh -NoProfile -File (Join-Path $PSScriptRoot 'backup-local.ps1') -BackupRoot $backupRoot -RetentionCount 2)
    Assert-LedgerSyncAcceptance ($LASTEXITCODE -eq 0) "Protected isolated backup failed."
    $backupLine = @($backupOutput | Where-Object { [string]$_ -like 'BACKUP_DIRECTORY=*' } | Select-Object -Last 1)
    Assert-LedgerSyncAcceptance ($backupLine.Count -eq 1) "Protected backup returned no exact directory."
    $backupDirectory = ([string]$backupLine[0]).Substring('BACKUP_DIRECTORY='.Length)
    $expectedBackupRoot = [IO.Path]::GetFullPath($backupRoot).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
    Assert-LedgerSyncAcceptance ([IO.Path]::GetFullPath($backupDirectory).StartsWith($expectedBackupRoot,[StringComparison]::OrdinalIgnoreCase)) "Backup directory escaped the isolated backup root."
    $validatedBackup = Assert-LedgerSyncBackupBundle -BackupDirectory $backupDirectory -BackupRoot $backupRoot
    Assert-LedgerSyncAcceptance ([string]$validatedBackup.Manifest.source_commit -ceq $commit -and [string]$validatedBackup.Manifest.schema.migration_version -ceq $migrationVersion) "Validated backup was not bound to the acceptance commit and schema."
    if ($env:OS -eq 'Windows_NT') {
        foreach ($protectedFile in @((Join-Path $backupDirectory 'manifest.json'),(Join-Path $backupDirectory 'database.dump'))) {
            Assert-LedgerSyncAcceptance ((Get-Acl -LiteralPath $protectedFile).AreAccessRulesProtected) "Validated backup file inherited host permissions."
        }
    }
    $restoreOutput = @(& pwsh -NoProfile -File (Join-Path $PSScriptRoot 'local-restore-drill.ps1') -ComposeProject $acceptanceProject -BackupDirectory $backupDirectory -BackupRoot $backupRoot -SkipCorruptionGuard)
    $restoreText = $restoreOutput -join ' '
    Assert-LedgerSyncAcceptance ($LASTEXITCODE -eq 0 -and $restoreText -match 'RESTORE_DRILL=PASS' -and $restoreText -match 'NORMAL_PROJECT_UNCHANGED=PASS' -and $restoreText -match 'REDIS_DBSIZE=[1-9][0-9]*') "Protected isolated restore did not pass all state proofs."
    $acceptancePassed = $true
}
catch {
    $failureMessage = ConvertTo-LedgerSyncRedactedLogLine -Line ([string]$_.Exception.Message)
}
finally {
    $PSNativeCommandUseErrorActionPreference = $false
    $cleanupFailures = [Collections.Generic.List[string]]::new()
    try {
        if ($acceptanceCreated) {
            Assert-LedgerSyncAcceptanceProjectIdentity -Project $acceptanceProject -StateRoot $acceptanceRoot -StatePath $acceptanceState
            $acceptanceEnvironment = Join-Path $acceptanceState 'runtime.env'
            if (Test-Path -LiteralPath $acceptanceEnvironment -PathType Leaf) {
                & docker compose --env-file $acceptanceEnvironment -p $acceptanceProject -f (Join-Path $repositoryRoot 'deploy\compose\docker-compose.yml') down --volumes --remove-orphans --timeout 10 *> $null
                Assert-LedgerSyncAcceptance ($LASTEXITCODE -eq 0) "Exact acceptance Compose cleanup failed."
            }
        }
    } catch { $cleanupFailures.Add((ConvertTo-LedgerSyncRedactedLogLine -Line ([string]$_.Exception.Message))) }
    try {
        Remove-LedgerSyncAcceptanceState
    } catch { $cleanupFailures.Add((ConvertTo-LedgerSyncRedactedLogLine -Line ([string]$_.Exception.Message))) }
    try {
        Remove-Item Env:LEDGERSYNC_LOCAL_COMPOSE_PROJECT -ErrorAction SilentlyContinue
        Remove-Item Env:LEDGERSYNC_LOCAL_STATE_DIRECTORY -ErrorAction SilentlyContinue
        if ($normalWasStopped) {
            & pwsh -NoProfile -File (Join-Path $PSScriptRoot 'start-local.ps1') | Out-Null
            Assert-LedgerSyncAcceptance ($LASTEXITCODE -eq 0) "Normal local stack could not be restored."
            . (Join-Path $PSScriptRoot 'local-runtime-common.ps1')
            . (Join-Path $PSScriptRoot 'local-acceptance-common.ps1')
            Assert-LedgerSyncLongRunningServicesHealthy
            Invoke-LedgerSyncWebSmoke
            Compare-LedgerSyncAcceptanceNormalState -Before $normalBefore -After (Get-LedgerSyncAcceptanceNormalState)
        }
    } catch { $cleanupFailures.Add((ConvertTo-LedgerSyncRedactedLogLine -Line ([string]$_.Exception.Message))) }
    try {
        Assert-LedgerSyncAcceptanceResourcesAbsent -Project $acceptanceProject
        Assert-LedgerSyncAcceptance (-not (Test-Path -LiteralPath $acceptanceState)) "Acceptance state directory remained after cleanup."
    } catch { $cleanupFailures.Add((ConvertTo-LedgerSyncRedactedLogLine -Line ([string]$_.Exception.Message))) }
    $cleanupPassed = $cleanupFailures.Count -eq 0
    if (-not $cleanupPassed -and [string]::IsNullOrWhiteSpace($failureMessage)) { $failureMessage = $cleanupFailures -join '; ' }
}

if (-not [string]::IsNullOrWhiteSpace($failureMessage)) { Write-Error "LOCAL_ACCEPTANCE=FAIL; $failureMessage"; exit 1 }
if (-not $acceptancePassed -or -not $cleanupPassed) { Write-Error 'LOCAL_ACCEPTANCE=FAIL; completion or cleanup proof is missing.'; exit 1 }
$elapsed = [Math]::Round(([DateTimeOffset]::UtcNow - $startedAt).TotalSeconds,2)
Write-Output 'LOCAL_ACCEPTANCE=PASS'
Write-Output "SOURCE_COMMIT=$commit"
Write-Output "MIGRATION_VERSION=$migrationVersion"
Write-Output 'SECURE_FRESH_STACK=PASS'
Write-Output 'ACCOUNT_LIFECYCLE=PASS'
Write-Output 'TRANSFER_IDEMPOTENCY=PASS'
Write-Output 'RECONCILIATION_IDEMPOTENCY=PASS'
Write-Output 'EVENT_DELIVERY_SEPARATION=PASS'
Write-Output 'EXACT_EXPORTS=PASS'
Write-Output 'DEPENDENCY_RESTARTS=PASS'
Write-Output 'PROTECTED_BACKUP_RESTORE=PASS'
if ($browserPassed) { Write-Output 'REAL_STACK_BROWSER=PASS' }
Write-Output 'NORMAL_PROJECT_RESTORED=PASS'
Write-Output 'ACCEPTANCE_CLEANUP=PASS'
if ($capacityPassed) { Write-Output 'CAPACITY=PASS' } else { Write-Output 'CAPACITY=SEPARATE_GATE' }
Write-Output 'SECURITY_SCAN=SEPARATE_GATE'
Write-Output "ELAPSED_SECONDS=$elapsed"
