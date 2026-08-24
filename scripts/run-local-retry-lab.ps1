[CmdletBinding()]
param(
    [switch]$ConfirmIsolatedRetryLab,
    [ValidateRange(100, 1000)][int64]$AmountMinor = 137
)

$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true
if (-not $ConfirmIsolatedRetryLab) {
    throw "The retry lab runs only in a new isolated project. Re-run with -ConfirmIsolatedRetryLab."
}

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$normalProject = "compose"
$acceptanceProject = "ledgersync-acceptance-$((Get-Date).ToUniversalTime().ToString('yyyyMMddHHmmss'))-$([Guid]::NewGuid().ToString('N').Substring(0, 8))"
$acceptanceRoot = [IO.Path]::GetFullPath((Join-Path $repositoryRoot "data\local-acceptance"))
$acceptanceState = [IO.Path]::GetFullPath((Join-Path $acceptanceRoot $acceptanceProject))
$sourceAccountID = "10000000-0000-4000-8000-000000000001"
$destinationAccountID = "10000000-0000-4000-8000-000000000002"
$tenantID = "00000000-0000-4000-8000-000000000001"
$idempotencyKey = [Guid]::NewGuid().ToString()
$normalWasStopped = $false
$acceptanceMayExist = $false
$normalBefore = $null
$failure = $null
$cleanupFailure = $null
$phase = "preflight"

. (Join-Path $PSScriptRoot "local-retry-lab-common.ps1")

function Assert-RetryLab {
    param([bool]$Condition, [Parameter(Mandatory = $true)][string]$Message)
    if (-not $Condition) { throw $Message }
}

function Assert-RetryLabIdentity {
    Assert-LedgerSyncRetryLabIdentity -RepositoryRoot $repositoryRoot `
        -Project $acceptanceProject -StateDirectory $acceptanceState | Out-Null
}

function Get-RetryLabProjectResources {
    $containers = @(& docker ps -a --filter "label=com.docker.compose.project=$acceptanceProject" --format '{{.ID}}')
    $volumes = @(& docker volume ls --filter "label=com.docker.compose.project=$acceptanceProject" --format '{{.Name}}')
    $networks = @(& docker network ls --filter "label=com.docker.compose.project=$acceptanceProject" --format '{{.ID}}')
    return [pscustomobject]@{
        Containers = @($containers | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) })
        Volumes = @($volumes | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) })
        Networks = @($networks | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) })
    }
}

function Remove-RetryLabState {
    if (-not (Test-Path -LiteralPath $acceptanceState)) { return }
    Assert-RetryLabIdentity
    $item = Get-Item -LiteralPath $acceptanceState -Force
    if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Retry lab cleanup refused a reparse-point state path."
    }
    Remove-Item -LiteralPath $acceptanceState -Recurse -Force
}

function Get-RetryLabDatabaseState {
    $sql = @"
SELECT json_build_object(
  'transfers',(SELECT count(*) FROM transfers),
  'journals',(SELECT count(*) FROM journal_transactions),
  'postings',(SELECT count(*) FROM ledger_postings),
  'source_available',(SELECT available_minor FROM account_balance_projections WHERE account_id='$sourceAccountID'),
  'source_ledger',(SELECT ledger_minor FROM account_balance_projections WHERE account_id='$sourceAccountID'),
  'source_version',(SELECT balance_version FROM account_balance_projections WHERE account_id='$sourceAccountID'),
  'destination_available',(SELECT available_minor FROM account_balance_projections WHERE account_id='$destinationAccountID'),
  'destination_ledger',(SELECT ledger_minor FROM account_balance_projections WHERE account_id='$destinationAccountID'),
  'destination_version',(SELECT balance_version FROM account_balance_projections WHERE account_id='$destinationAccountID')
)::text;
"@
    $output = @(Invoke-LedgerSyncCompose -ComposeArguments @(
        "exec", "-T", "postgres", "psql", "-U", "ledgersync", "-d", "ledgersync", "-Atc", $sql
    ) -CaptureOutput)
    $json = @($output | Where-Object { ([string]$_).TrimStart().StartsWith("{") } | Select-Object -Last 1)
    if ($json.Count -ne 1) { throw "Retry lab database state did not return one bounded result." }
    return ([string]$json[0] | ConvertFrom-Json)
}

function Get-RetryLabIdempotencyEvidence {
    if ($idempotencyKey -cnotmatch '^[0-9a-f-]{36}$') { throw "Retry lab idempotency identity is invalid." }
    $sql = @"
WITH request AS (
  SELECT transfer_id,state,response_status,response_body
  FROM idempotency_requests
  WHERE tenant_id='$tenantID' AND actor_subject_id='demo-operator'
    AND operation='transfers.create.v1' AND idempotency_key='$idempotencyKey'
), target AS (
  SELECT transfer_id FROM request
)
SELECT json_build_object(
  'request_count',(SELECT count(*) FROM request),
  'state',(SELECT state FROM request),
  'response_status',(SELECT response_status FROM request),
  'transfer_id',(SELECT transfer_id FROM request),
  'response_transfer_id',(SELECT response_body->>'transfer_id' FROM request),
  'transfer_count',(SELECT count(*) FROM transfers WHERE id=(SELECT transfer_id FROM target)),
  'journal_count',(SELECT count(*) FROM journal_transactions WHERE transfer_id=(SELECT transfer_id FROM target)),
  'posting_count',(SELECT count(*) FROM ledger_postings WHERE journal_transaction_id=(SELECT journal_transaction_id FROM transfers WHERE id=(SELECT transfer_id FROM target)))
)::text;
"@
    $output = @(Invoke-LedgerSyncCompose -ComposeArguments @(
        "exec", "-T", "postgres", "psql", "-U", "ledgersync", "-d", "ledgersync", "-Atc", $sql
    ) -CaptureOutput)
    $json = @($output | Where-Object { ([string]$_).TrimStart().StartsWith("{") } | Select-Object -Last 1)
    if ($json.Count -ne 1) { throw "Retry lab idempotency evidence did not return one bounded result." }
    return ([string]$json[0] | ConvertFrom-Json)
}

function Invoke-RetryLabLostResponseBoundary {
    param(
        [Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session,
        [Parameter(Mandatory = $true)][string]$CSRFToken,
        [Parameter(Mandatory = $true)][string]$Body
    )
    $response = Invoke-WebRequest -UseBasicParsing -WebSession $Session -TimeoutSec 15 `
        -Method Post -Uri "$script:LedgerSyncWebUrl/api/transfers" `
        -Headers @{ Origin = $script:LedgerSyncWebUrl; "X-CSRF-Token" = $CSRFToken; "Idempotency-Key" = $idempotencyKey } `
        -ContentType "application/json" -Body $Body
    Assert-RetryLab ([int]$response.StatusCode -eq 201) "Retry lab first request did not commit."
    Assert-RetryLab ([string]$response.Headers["Idempotent-Replay"] -ine "true") "Retry lab first request was unexpectedly a replay."
    # Deliberately discard the entire successful response at the harness
    # boundary. No server, proxy, database, or browser fault toggle is changed.
    $response = $null
    throw "LEDGERSYNC_RETRY_LAB_RESPONSE_LOST_AFTER_COMMIT"
}

try {
    Assert-RetryLabIdentity
    if (-not [string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable("LEDGERSYNC_LOCAL_COMPOSE_PROJECT")) -or
        -not [string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable("LEDGERSYNC_LOCAL_STATE_DIRECTORY"))) {
        throw "Retry lab requires unset project/state overrides."
    }
    . (Join-Path $PSScriptRoot "local-runtime-common.ps1")
    . (Join-Path $PSScriptRoot "local-acceptance-common.ps1")
    Assert-LedgerSyncDockerAvailable
    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy
    Invoke-LedgerSyncWebSmoke
    $normalBefore = Get-LedgerSyncFinancialFingerprint
    $collision = Get-RetryLabProjectResources
    Assert-RetryLab ($collision.Containers.Count -eq 0 -and $collision.Volumes.Count -eq 0 -and
        $collision.Networks.Count -eq 0 -and -not (Test-Path -LiteralPath $acceptanceState)) "Retry lab project/state collision detected."

    $phase = "normal project stop"
    & pwsh -NoProfile -File (Join-Path $PSScriptRoot "stop-local.ps1") *> $null
    if ($LASTEXITCODE -ne 0) { throw "Retry lab could not safely stop the normal project." }
    $normalWasStopped = $true

    $phase = "isolated project startup"
    $env:LEDGERSYNC_LOCAL_COMPOSE_PROJECT = $acceptanceProject
    $env:LEDGERSYNC_LOCAL_STATE_DIRECTORY = $acceptanceState
    . (Join-Path $PSScriptRoot "local-runtime-common.ps1")
    . (Join-Path $PSScriptRoot "local-acceptance-common.ps1")
    New-Item -ItemType Directory -Path $acceptanceRoot -Force | Out-Null
    $acceptanceMayExist = $true
    & pwsh -NoProfile -File (Join-Path $PSScriptRoot "start-local.ps1") -InitializationMode demo *> $null
    if ($LASTEXITCODE -ne 0) { throw "Retry lab isolated demo project did not start." }
    Assert-LedgerSyncOneShotServicesCompleted
    Assert-LedgerSyncLongRunningServicesHealthy
    Invoke-LedgerSyncWebSmoke

    $session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
    $sessionPayload = Invoke-LedgerSyncAcceptanceGET -Session $session -Path "/api/session"
    $csrf = [string]$sessionPayload.csrf_token
    Assert-RetryLab ($csrf.Length -ge 32 -and [string]$sessionPayload.tenant_id -ceq $tenantID) "Retry lab demo session is invalid."
    $body = [ordered]@{
        sourceAccountId = $sourceAccountID
        destinationAccountId = $destinationAccountID
        amount = [ordered]@{ currency = "INR"; minorUnits = [string]$AmountMinor }
    } | ConvertTo-Json -Depth 3 -Compress
    $before = Get-RetryLabDatabaseState

    $phase = "first committed request"
    $lost = $false
    try {
        Invoke-RetryLabLostResponseBoundary -Session $session -CSRFToken $csrf -Body $body
    }
    catch {
        $lost = $_.Exception.Message -ceq "LEDGERSYNC_RETRY_LAB_RESPONSE_LOST_AFTER_COMMIT"
        if (-not $lost) {
            $status = "none"
            $responseProperty = $_.Exception.PSObject.Properties["Response"]
            if ($null -ne $responseProperty -and $null -ne $responseProperty.Value) {
                $status = [string][int]$responseProperty.Value.StatusCode
            }
            $phase = "first committed request http_status=$status"
            throw "Retry lab first request failed before the simulated lost-response boundary (http_status=$status)."
        }
    }
    Assert-RetryLab $lost "Retry lab did not stop at the deliberate lost-response boundary."
    $phase = "post-commit database proof"
    $afterCommit = Get-RetryLabDatabaseState
    $committed = Get-RetryLabIdempotencyEvidence
    Assert-RetryLab ([int64]$afterCommit.transfers -eq [int64]$before.transfers + 1) "Lost-response commit did not create exactly one transfer."
    Assert-RetryLab ([int64]$afterCommit.journals -eq [int64]$before.journals + 1) "Lost-response commit did not create exactly one journal."
    Assert-RetryLab ([int64]$afterCommit.postings -eq [int64]$before.postings + 2) "Lost-response commit did not create exactly two postings."
    Assert-RetryLab ([int64]$before.source_available - [int64]$afterCommit.source_available -eq $AmountMinor) "Source balance was not debited exactly once."
    Assert-RetryLab ([int64]$afterCommit.destination_available - [int64]$before.destination_available -eq $AmountMinor) "Destination balance was not credited exactly once."
    Assert-RetryLab ([int64]$before.source_ledger - [int64]$afterCommit.source_ledger -eq $AmountMinor -and
        [int64]$afterCommit.destination_ledger - [int64]$before.destination_ledger -eq $AmountMinor) "Ledger balances did not move exactly once."
    Assert-RetryLab ([int64]$afterCommit.source_version -eq [int64]$before.source_version + 1 -and
        [int64]$afterCommit.destination_version -eq [int64]$before.destination_version + 1) "Balance versions did not advance exactly once."
    Assert-RetryLab ([int]$committed.request_count -eq 1 -and $committed.state -ceq "completed" -and
        [int]$committed.response_status -eq 201 -and [int]$committed.transfer_count -eq 1 -and
        [int]$committed.journal_count -eq 1 -and [int]$committed.posting_count -eq 2) "Committed idempotency evidence is incomplete or duplicated."
    $committedTransferID = [string]$committed.transfer_id
    Assert-RetryLab ($committedTransferID -cmatch '^[0-9a-f-]{36}$' -and
        [string]$committed.response_transfer_id -ceq $committedTransferID) "Stored idempotent response is not bound to the committed transfer."

    $phase = "same-key identical retry"
    $retryResponse = Invoke-WebRequest -UseBasicParsing -WebSession $session -TimeoutSec 15 `
        -Method Post -Uri "$script:LedgerSyncWebUrl/api/transfers" `
        -Headers @{ Origin = $script:LedgerSyncWebUrl; "X-CSRF-Token" = $csrf; "Idempotency-Key" = $idempotencyKey } `
        -ContentType "application/json" -Body $body
    Assert-RetryLab ([int]$retryResponse.StatusCode -eq 201 -and
        [string]$retryResponse.Headers["Idempotent-Replay"] -ieq "true") "Identical retry was not returned as an idempotent replay."
    $retryPayload = $retryResponse.Content | ConvertFrom-Json
    Assert-RetryLab ([string]$retryPayload.transfer_id -ceq $committedTransferID) "Identical retry returned a different transfer ID."
    $afterRetry = Get-RetryLabDatabaseState
    foreach ($field in @("transfers", "journals", "postings", "source_available", "source_ledger", "source_version", "destination_available", "destination_ledger", "destination_version")) {
        Assert-RetryLab ([string]$afterRetry.$field -ceq [string]$afterCommit.$field) "Identical retry changed '$field'."
    }
    $replayed = Get-RetryLabIdempotencyEvidence
    Assert-RetryLab ([string]$replayed.transfer_id -ceq $committedTransferID -and
        [int]$replayed.request_count -eq 1 -and [int]$replayed.transfer_count -eq 1 -and
        [int]$replayed.journal_count -eq 1 -and [int]$replayed.posting_count -eq 2) "Replay changed committed database cardinality."
    $detail = Invoke-LedgerSyncAcceptanceGET -Session $session -Path "/api/transfers/$committedTransferID"
    Assert-RetryLab ([string]$detail.financial_status -ceq "posted" -and
        @($detail.postings).Count -eq 2) "Stored transfer detail does not show one balanced journal."

    $phase = "isolated reconciliation"
    $summary = Invoke-LedgerSyncAcceptanceReconciliation -TenantID $tenantID
    Assert-RetryLab ([int64]$summary.reconciliation_mismatches -eq 0) "Retry lab reconciliation reported a mismatch."

    Write-Output "ISOLATED_SAME_KEY_RETRY_LAB=PASS"
    Write-Output "LOST_RESPONSE_BOUNDARY=CLIENT_HARNESS_ONLY"
    Write-Output "UNCHANGED_TRANSFER_ID=PASS"
    Write-Output "CARDINALITY=transfers:1,journals:1,postings:2"
    Write-Output "EXACT_BALANCE_DELTA_MINOR=$AmountMinor"
    Write-Output "RECONCILIATION=status:$($summary.reconciliation_status),mismatches:0"
}
catch {
    $failure = $_
}
finally {
    $PSNativeCommandUseErrorActionPreference = $false
    try {
        if ($acceptanceMayExist -and $acceptanceProject -cmatch '^ledgersync-acceptance-\d{14}-[0-9a-f]{8}$') {
            $runtimePath = Join-Path $acceptanceState "runtime.env"
            if (Test-Path -LiteralPath $runtimePath -PathType Leaf) {
                & docker compose --env-file $runtimePath -p $acceptanceProject `
                    -f (Join-Path $repositoryRoot "deploy\compose\docker-compose.yml") `
                    down --volumes --remove-orphans --timeout 10 *> $null
            }
        }
        Remove-RetryLabState
        $remaining = Get-RetryLabProjectResources
        if ($remaining.Containers.Count -ne 0 -or $remaining.Volumes.Count -ne 0 -or
            $remaining.Networks.Count -ne 0 -or (Test-Path -LiteralPath $acceptanceState)) {
            throw "Retry lab cleanup left isolated project resources or state."
        }
        Write-Output "RETRY_LAB_CLEANUP=PASS"
    }
    catch {
        $cleanupFailure = $_
    }

    Remove-Item Env:LEDGERSYNC_LOCAL_COMPOSE_PROJECT -ErrorAction SilentlyContinue
    Remove-Item Env:LEDGERSYNC_LOCAL_STATE_DIRECTORY -ErrorAction SilentlyContinue
    Remove-Item Env:LEDGERSYNC_INITIALIZATION_MODE -ErrorAction SilentlyContinue
    if ($normalWasStopped) {
        try {
            & pwsh -NoProfile -File (Join-Path $PSScriptRoot "start-local.ps1") -SkipBuild *> $null
            if ($LASTEXITCODE -ne 0) { throw "Normal project did not restart." }
            . (Join-Path $PSScriptRoot "local-runtime-common.ps1")
            Assert-LedgerSyncLongRunningServicesHealthy
            Invoke-LedgerSyncWebSmoke
            Compare-LedgerSyncFinancialFingerprint -Before $normalBefore -After (Get-LedgerSyncFinancialFingerprint)
            Write-Output "NORMAL_PROJECT_RESTORED=PASS"
        }
        catch {
            if ($null -eq $cleanupFailure) { $cleanupFailure = $_ }
        }
    }
}

if ($null -ne $cleanupFailure) {
    throw "Retry lab cleanup or normal-project restoration failed; inspect only the exact generated project."
}
if ($null -ne $failure) {
    throw "Retry lab failed during $phase; isolated cleanup and normal restoration were attempted."
}
