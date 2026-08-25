Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$script:LedgerSyncAcceptanceProjectPattern = '^ledgersync-acceptance-\d{14}-[0-9a-f]{8}$'
$script:LedgerSyncAcceptanceUUIDPattern = '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'

function Assert-LedgerSyncAcceptance {
    param([bool]$Condition, [Parameter(Mandatory = $true)][string]$Message)
    if (-not $Condition) { throw $Message }
}

function Invoke-LedgerSyncAcceptanceGET {
    param(
        [Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session,
        [Parameter(Mandatory = $true)][string]$Path
    )
    $response = Invoke-WebRequest -UseBasicParsing -WebSession $Session -TimeoutSec 15 `
        -Uri "$script:LedgerSyncWebUrl$Path"
    Assert-LedgerSyncAcceptance ([int]$response.StatusCode -eq 200) "Acceptance GET $Path returned HTTP $([int]$response.StatusCode)."
    return ($response.Content | ConvertFrom-Json)
}

function Get-LedgerSyncAcceptanceBalance {
    param(
        [Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session,
        [Parameter(Mandatory = $true)][string]$AccountID
    )
    $balance = Invoke-LedgerSyncAcceptanceGET -Session $Session -Path "/api/accounts/$AccountID/balance"
    Assert-LedgerSyncAcceptance ([string]$balance.account_id -ceq $AccountID) "Balance response belongs to an unexpected account."
    Assert-LedgerSyncAcceptance ([string]$balance.currency -ceq "INR") "Acceptance balance is not exact INR evidence."
    return $balance
}

function Invoke-LedgerSyncAcceptanceTransfer {
    param(
        [Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session,
        [Parameter(Mandatory = $true)][string]$CSRFToken,
        [Parameter(Mandatory = $true)][string]$SourceAccountID,
        [Parameter(Mandatory = $true)][string]$DestinationAccountID,
        [Parameter(Mandatory = $true)][string]$AmountMinor,
        [Parameter(Mandatory = $true)][string]$IdempotencyKey,
        [switch]$ExpectReplay
    )
    $body = @{
        sourceAccountId = $SourceAccountID
        destinationAccountId = $DestinationAccountID
        amount = @{ currency = "INR"; minorUnits = $AmountMinor }
    } | ConvertTo-Json -Depth 3 -Compress
    $response = Invoke-WebRequest -UseBasicParsing -WebSession $Session -TimeoutSec 15 `
        -Method Post -Uri "$script:LedgerSyncWebUrl/api/transfers" `
        -Headers @{
            Origin = $script:LedgerSyncWebUrl
            "X-CSRF-Token" = $CSRFToken
            "Idempotency-Key" = $IdempotencyKey
        } -ContentType "application/json" -Body $body
    Assert-LedgerSyncAcceptance ([int]$response.StatusCode -eq 201) "Acceptance transfer returned HTTP $([int]$response.StatusCode)."
    $payload = $response.Content | ConvertFrom-Json
    Assert-LedgerSyncAcceptance ([string]$payload.status -ceq "posted") "Acceptance transfer did not post."
    Assert-LedgerSyncAcceptance ([string]$payload.currency -ceq "INR") "Acceptance transfer changed currency."
    Assert-LedgerSyncAcceptance ([string]$payload.amount_minor -ceq $AmountMinor) "Acceptance transfer changed the exact minor-unit amount."
    Assert-LedgerSyncAcceptance ([string]$payload.transfer_id -cmatch '^[0-9a-f-]{36}$') "Acceptance transfer returned no valid identifier."
    $replayHeader = [string]$response.Headers["Idempotent-Replay"]
    if ($ExpectReplay) {
        Assert-LedgerSyncAcceptance ($replayHeader -ieq "true") "Same-key acceptance retry was not identified as an idempotent replay."
    } else {
        Assert-LedgerSyncAcceptance ($replayHeader -ine "true") "A new acceptance intent was unexpectedly reported as a replay."
    }
    return $payload
}

function Test-LedgerSyncAcceptanceJourney {
    param(
        [Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session,
        [Parameter(Mandatory = $true)][string]$CSRFToken,
        [Parameter(Mandatory = $true)][string]$AmountMinor,
        [switch]$VerifyReplay
    )
    $sourceID = "10000000-0000-4000-8000-000000000001"
    $destinationID = "10000000-0000-4000-8000-000000000002"
    $accounts = Invoke-LedgerSyncAcceptanceGET -Session $Session -Path "/api/me/accounts?limit=10"
    $authorizedIDs = @($accounts.accounts | ForEach-Object { [string]$_.account_id })
    Assert-LedgerSyncAcceptance ($authorizedIDs -contains $sourceID -and $authorizedIDs -contains $destinationID) "Acceptance account directory omitted an authorized transfer account."
    Invoke-LedgerSyncAcceptanceGET -Session $Session -Path "/api/accounts/$sourceID" | Out-Null

    $sourceBefore = Get-LedgerSyncAcceptanceBalance -Session $Session -AccountID $sourceID
    $destinationBefore = Get-LedgerSyncAcceptanceBalance -Session $Session -AccountID $destinationID
    $key = [Guid]::NewGuid().ToString()
    $posted = Invoke-LedgerSyncAcceptanceTransfer -Session $Session -CSRFToken $CSRFToken `
        -SourceAccountID $sourceID -DestinationAccountID $destinationID `
        -AmountMinor $AmountMinor -IdempotencyKey $key
    $sourceAfter = Get-LedgerSyncAcceptanceBalance -Session $Session -AccountID $sourceID
    $destinationAfter = Get-LedgerSyncAcceptanceBalance -Session $Session -AccountID $destinationID
    Assert-LedgerSyncAcceptance (([int64]$sourceBefore.available_minor - [int64]$sourceAfter.available_minor) -eq [int64]$AmountMinor) "Source account did not move by the exact debit amount."
    Assert-LedgerSyncAcceptance (([int64]$destinationAfter.available_minor - [int64]$destinationBefore.available_minor) -eq [int64]$AmountMinor) "Destination account did not move by the exact credit amount."
    Assert-LedgerSyncAcceptance ([int64]$sourceAfter.version -gt [int64]$sourceBefore.version) "Source balance version did not advance."
    Assert-LedgerSyncAcceptance ([int64]$destinationAfter.version -gt [int64]$destinationBefore.version) "Destination balance version did not advance."

    if ($VerifyReplay) {
        $replayed = Invoke-LedgerSyncAcceptanceTransfer -Session $Session -CSRFToken $CSRFToken `
            -SourceAccountID $sourceID -DestinationAccountID $destinationID `
            -AmountMinor $AmountMinor -IdempotencyKey $key -ExpectReplay
        Assert-LedgerSyncAcceptance ([string]$replayed.transfer_id -ceq [string]$posted.transfer_id) "Same-key retry returned a different transfer identifier."
        $sourceReplay = Get-LedgerSyncAcceptanceBalance -Session $Session -AccountID $sourceID
        $destinationReplay = Get-LedgerSyncAcceptanceBalance -Session $Session -AccountID $destinationID
        Assert-LedgerSyncAcceptance ([int64]$sourceReplay.available_minor -eq [int64]$sourceAfter.available_minor) "Same-key retry debited the source twice."
        Assert-LedgerSyncAcceptance ([int64]$destinationReplay.available_minor -eq [int64]$destinationAfter.available_minor) "Same-key retry credited the destination twice."
    }

    $transferID = [string]$posted.transfer_id
    $detail = Invoke-LedgerSyncAcceptanceGET -Session $Session -Path "/api/transfers/$transferID"
    Assert-LedgerSyncAcceptance ([string]$detail.financial_status -ceq "posted") "Transfer detail did not preserve posted financial truth."
    Assert-LedgerSyncAcceptance ([string]$detail.journal_transaction_id -cmatch '^[0-9a-f-]{36}$') "Transfer detail omitted its immutable journal identifier."
    Assert-LedgerSyncAcceptance (@($detail.postings).Count -eq 2) "Transfer detail did not contain exactly two ledger postings."
    $debit = [int64](@($detail.postings | Where-Object direction -eq "debit")[0].amount_minor)
    $credit = [int64](@($detail.postings | Where-Object direction -eq "credit")[0].amount_minor)
    Assert-LedgerSyncAcceptance ($debit -eq [int64]$AmountMinor -and $credit -eq [int64]$AmountMinor) "Double-entry postings are not balanced to the exact amount."
    $history = Invoke-LedgerSyncAcceptanceGET -Session $Session -Path "/api/accounts/$sourceID/transactions?limit=20"
    Assert-LedgerSyncAcceptance (@($history.transactions | Where-Object transfer_id -eq $transferID).Count -eq 1) "Source transaction history omitted or duplicated the posted transfer."
    return $transferID
}

function Invoke-LedgerSyncAcceptanceReconciliation {
    param([Parameter(Mandatory = $true)][string]$TenantID)
    $output = @(Invoke-LedgerSyncCompose -ComposeArguments @(
        "run", "--rm", "--no-deps", "--entrypoint", "/usr/local/bin/reconcile", "api",
        "--run", "--rebuild-cache", "--tenant-id", $TenantID
    ) -CaptureOutput)
    Assert-LedgerSyncAcceptance (($output -join " ") -match 'status=matched') "Acceptance reconciliation did not match."
    Assert-LedgerSyncAcceptance (($output -join " ") -match 'mismatch_count=0') "Acceptance reconciliation reported a mismatch."
    $summary = Get-LedgerSyncOperationalSummary
    Assert-LedgerSyncAcceptance ([int64]$summary.reconciliation_mismatches -eq 0) "Operational summary reported reconciliation drift."
    Assert-LedgerSyncAcceptance ($summary.reconciliation_status -in @("completed", "matched", "passed")) "Operational reconciliation status is not healthy."
    return $summary
}

function Assert-LedgerSyncAcceptanceProjectIdentity {
    param(
        [Parameter(Mandatory = $true)][string]$Project,
        [Parameter(Mandatory = $true)][string]$StateRoot,
        [Parameter(Mandatory = $true)][string]$StatePath
    )
    Assert-LedgerSyncAcceptance ($Project -cmatch $script:LedgerSyncAcceptanceProjectPattern) "Acceptance project identity is not canonical."
    $canonicalRoot = [IO.Path]::GetFullPath($StateRoot).TrimEnd([IO.Path]::DirectorySeparatorChar)
    $canonicalState = [IO.Path]::GetFullPath($StatePath).TrimEnd([IO.Path]::DirectorySeparatorChar)
    $expected = [IO.Path]::GetFullPath((Join-Path $canonicalRoot $Project)).TrimEnd([IO.Path]::DirectorySeparatorChar)
    Assert-LedgerSyncAcceptance ($canonicalState.Equals($expected, [StringComparison]::OrdinalIgnoreCase)) "Acceptance state is outside its exact project directory."
    $cursor = $canonicalState
    while ($cursor.Length -ge $canonicalRoot.Length) {
        if (Test-Path -LiteralPath $cursor) {
            $item = Get-Item -LiteralPath $cursor -Force
            Assert-LedgerSyncAcceptance (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -eq 0) "Acceptance state ancestry contains a reparse point."
        }
        if ($cursor.Equals($canonicalRoot, [StringComparison]::OrdinalIgnoreCase)) { break }
        $parent = Split-Path -Parent $cursor
        Assert-LedgerSyncAcceptance (-not [string]::IsNullOrWhiteSpace($parent) -and $parent.Length -lt $cursor.Length) "Acceptance state ancestry could not be resolved."
        $cursor = $parent
    }
}

function Invoke-LedgerSyncAcceptanceJSON {
    param(
        [Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session,
        [Parameter(Mandatory = $true)][ValidateSet('GET','POST','PATCH')][string]$Method,
        [Parameter(Mandatory = $true)][string]$Path,
        [string]$CSRFToken = '',
        [string]$IdempotencyKey = '',
        [AllowNull()][object]$Body = $null,
        [int[]]$ExpectedStatus = @(200)
    )
    $headers = @{ Accept = 'application/json' }
    if ($Method -ne 'GET') {
        Assert-LedgerSyncAcceptance ($CSRFToken.Length -ge 32) "Acceptance mutation requires a bounded CSRF token."
        Assert-LedgerSyncAcceptance ($IdempotencyKey.Length -ge 16 -and $IdempotencyKey.Length -le 255) "Acceptance mutation requires a valid idempotency key."
        $headers.Origin = $script:LedgerSyncWebUrl
        $headers.'X-CSRF-Token' = $CSRFToken
        $headers.'Idempotency-Key' = $IdempotencyKey
    }
    $arguments = @{
        UseBasicParsing = $true; WebSession = $Session; TimeoutSec = 20
        Method = $Method; Uri = "$script:LedgerSyncWebUrl$Path"; Headers = $headers
        SkipHttpErrorCheck = $true
    }
    if ($null -ne $Body) {
        $arguments.ContentType = 'application/json'
        $arguments.Body = ($Body | ConvertTo-Json -Depth 5 -Compress)
    }
    $response = Invoke-WebRequest @arguments
    $status = [int]$response.StatusCode
    Assert-LedgerSyncAcceptance ($ExpectedStatus -contains $status) "Acceptance request returned an unexpected HTTP status for $Method $Path."
    Assert-LedgerSyncAcceptance ([string]$response.Headers['Cache-Control'] -match '(^|,)\s*no-store\s*(,|$)') "Acceptance response omitted no-store."
    Assert-LedgerSyncAcceptance ([Text.Encoding]::UTF8.GetByteCount([string]$response.Content) -le 262144) "Acceptance response exceeded its evidence bound."
    $payload = if ([string]::IsNullOrWhiteSpace([string]$response.Content)) { $null } else { $response.Content | ConvertFrom-Json }
    return [pscustomobject]@{ Status = $status; Headers = $response.Headers; Payload = $payload }
}

function Invoke-LedgerSyncAcceptanceSQLJSON {
    param([Parameter(Mandatory = $true)][string]$SQL)
    $output = @(Invoke-LedgerSyncCompose -ComposeArguments @(
        'exec','-T','postgres','psql','-v','ON_ERROR_STOP=1','-U','ledgersync','-d','ledgersync','-Atc',$SQL
    ) -CaptureOutput)
    $json = @($output | Where-Object { ([string]$_).TrimStart().StartsWith('{') } | Select-Object -Last 1)
    Assert-LedgerSyncAcceptance ($json.Count -eq 1) "Acceptance database evidence was not one bounded JSON record."
    return ([string]$json[0] | ConvertFrom-Json)
}

function Test-LedgerSyncAcceptanceAccountCreationBoundary {
    param(
        [Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session,
        [Parameter(Mandatory = $true)][string]$CSRFToken,
        [Parameter(Mandatory = $true)][string]$RunLabel
    )
    Assert-LedgerSyncAcceptance ($RunLabel -cmatch '^[a-z0-9]{8,20}$') "Acceptance account run label is invalid."
    $key = "account-create-$RunLabel"
    $body = @{ display_name = "Acceptance $RunLabel"; external_reference = "accept-$RunLabel"; category = 'operating'; currency = 'INR' }
    $first = Invoke-LedgerSyncAcceptanceJSON -Session $Session -Method POST -Path '/api/me/accounts' -CSRFToken $CSRFToken -IdempotencyKey $key -Body $body -ExpectedStatus 201
    $accountID = [string]$first.Payload.account_id
    Assert-LedgerSyncAcceptance ($accountID -cmatch $script:LedgerSyncAcceptanceUUIDPattern) "Account creation returned no canonical identifier."
    Assert-LedgerSyncAcceptance ([string]$first.Payload.account_version -ceq '1' -and [string]$first.Payload.available_minor -ceq '0' -and [string]$first.Payload.ledger_minor -ceq '0') "New account did not preserve exact version and zero-money strings."
    $first = $null # Deliberately discard the client response at the harness boundary.
    $database = Invoke-LedgerSyncAcceptanceSQLJSON -SQL @"
SELECT json_build_object(
 'accounts',(SELECT count(*) FROM accounts WHERE id='$accountID'::uuid AND external_reference='accept-$RunLabel'),
 'idempotency',(SELECT count(*) FROM idempotency_requests WHERE operation='accounts.create.v1' AND idempotency_key='$key' AND state='completed'),
 'audit',(SELECT count(*) FROM audit_events WHERE target_id='$accountID' AND event_type='account.created' AND outcome='succeeded'),
 'outbox',(SELECT count(*) FROM outbox_events WHERE account_id='$accountID'::uuid AND event_type='account.created.v1'),
 'available',(SELECT available_minor FROM account_balance_projections WHERE account_id='$accountID'::uuid),
 'ledger',(SELECT ledger_minor FROM account_balance_projections WHERE account_id='$accountID'::uuid)
);
"@
    Assert-LedgerSyncAcceptance ([int]$database.accounts -eq 1 -and [int]$database.idempotency -eq 1 -and [int]$database.audit -eq 1 -and [int]$database.outbox -eq 1) "Account response-boundary commit was not atomic and exactly once."
    Assert-LedgerSyncAcceptance ([int64]$database.available -eq 0 -and [int64]$database.ledger -eq 0) "Created account did not have exact zero database balances."
    $replay = Invoke-LedgerSyncAcceptanceJSON -Session $Session -Method POST -Path '/api/me/accounts' -CSRFToken $CSRFToken -IdempotencyKey $key -Body $body -ExpectedStatus 201
    Assert-LedgerSyncAcceptance ([string]$replay.Payload.account_id -ceq $accountID -and [string]$replay.Headers['Idempotent-Replay'] -ieq 'true') "Account response-boundary retry did not replay the committed identity."
    $changed = $body.Clone(); $changed.display_name = "Changed $RunLabel"
    $conflict = Invoke-LedgerSyncAcceptanceJSON -Session $Session -Method POST -Path '/api/me/accounts' -CSRFToken $CSRFToken -IdempotencyKey $key -Body $changed -ExpectedStatus 409
    Assert-LedgerSyncAcceptance ([string]$conflict.Payload.error.code -ceq 'idempotency_conflict') "Changed account intent did not produce a stable idempotency conflict."
    return [pscustomobject]@{ AccountID = $accountID; Key = $key; Body = $body }
}

function Invoke-LedgerSyncAcceptanceAccountStatus {
    param(
        [Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session,
        [Parameter(Mandatory = $true)][string]$CSRFToken,
        [Parameter(Mandatory = $true)][string]$AccountID,
        [Parameter(Mandatory = $true)][string]$ExpectedVersion,
        [Parameter(Mandatory = $true)][ValidateSet('active','frozen','closed')][string]$TargetStatus,
        [Parameter(Mandatory = $true)][string]$Reason,
        [Parameter(Mandatory = $true)][string]$IdempotencyKey,
        [int]$ExpectedStatus = 200,
        [string]$ExpectedErrorCode = '',
        [switch]$ExpectReplay
    )
    $body = @{ expected_version = $ExpectedVersion; target_status = $TargetStatus; reason = $Reason }
    $response = Invoke-LedgerSyncAcceptanceJSON -Session $Session -Method PATCH -Path "/api/accounts/$AccountID" -CSRFToken $CSRFToken -IdempotencyKey $IdempotencyKey -Body $body -ExpectedStatus $ExpectedStatus
    if ($ExpectedStatus -ge 400) {
        Assert-LedgerSyncAcceptance ([string]$response.Payload.error.code -ceq $ExpectedErrorCode) "Account lifecycle denial did not preserve its stable public code."
    } else {
        Assert-LedgerSyncAcceptance ([string]$response.Payload.account_id -ceq $AccountID -and [string]$response.Payload.status -ceq $TargetStatus) "Account lifecycle response did not preserve the requested target."
    }
    if ($ExpectReplay) { Assert-LedgerSyncAcceptance ([string]$response.Headers['Idempotent-Replay'] -ieq 'true') "Account lifecycle retry was not marked as a replay." }
    return $response
}

function Invoke-LedgerSyncAcceptanceTransferOutcome {
    param(
        [Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session,
        [Parameter(Mandatory = $true)][string]$CSRFToken,
        [Parameter(Mandatory = $true)][string]$SourceAccountID,
        [Parameter(Mandatory = $true)][string]$DestinationAccountID,
        [Parameter(Mandatory = $true)][string]$AmountMinor,
        [Parameter(Mandatory = $true)][string]$IdempotencyKey,
        [int]$ExpectedStatus = 201,
        [string]$ExpectedErrorCode = '',
        [switch]$ExpectReplay
    )
    $body = @{ sourceAccountId = $SourceAccountID; destinationAccountId = $DestinationAccountID; amount = @{ currency = 'INR'; minorUnits = $AmountMinor } }
    $response = Invoke-LedgerSyncAcceptanceJSON -Session $Session -Method POST -Path '/api/transfers' -CSRFToken $CSRFToken -IdempotencyKey $IdempotencyKey -Body $body -ExpectedStatus $ExpectedStatus
    if ($ExpectedStatus -ge 400) {
        Assert-LedgerSyncAcceptance ([string]$response.Payload.error.code -ceq $ExpectedErrorCode) "Transfer denial did not preserve its stable public code."
    } else {
        Assert-LedgerSyncAcceptance ([string]$response.Payload.status -ceq 'posted' -and [string]$response.Payload.currency -ceq 'INR' -and [string]$response.Payload.amount_minor -ceq $AmountMinor) "Transfer result did not preserve exact posted money."
        Assert-LedgerSyncAcceptance ([string]$response.Payload.transfer_id -cmatch $script:LedgerSyncAcceptanceUUIDPattern) "Transfer returned no canonical identifier."
    }
    if ($ExpectReplay) { Assert-LedgerSyncAcceptance ([string]$response.Headers['Idempotent-Replay'] -ieq 'true') "Transfer retry was not marked as a replay." }
    return [pscustomobject]@{ Response = $response; Body = $body }
}

function Test-LedgerSyncAcceptanceLostTransferBoundary {
    param(
        [Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session,
        [Parameter(Mandatory = $true)][string]$CSRFToken,
        [Parameter(Mandatory = $true)][string]$SourceAccountID,
        [Parameter(Mandatory = $true)][string]$DestinationAccountID,
        [Parameter(Mandatory = $true)][string]$AmountMinor,
        [Parameter(Mandatory = $true)][string]$IdempotencyKey
    )
    $sourceBefore = Get-LedgerSyncAcceptanceBalance -Session $Session -AccountID $SourceAccountID
    $destinationBefore = Get-LedgerSyncAcceptanceBalance -Session $Session -AccountID $DestinationAccountID
    $first = Invoke-LedgerSyncAcceptanceTransferOutcome -Session $Session -CSRFToken $CSRFToken -SourceAccountID $SourceAccountID -DestinationAccountID $DestinationAccountID -AmountMinor $AmountMinor -IdempotencyKey $IdempotencyKey
    $transferID = [string]$first.Response.Payload.transfer_id
    $body = $first.Body
    $first = $null # Lost only at the client/harness boundary, after the server committed.
    $database = Invoke-LedgerSyncAcceptanceSQLJSON -SQL @"
SELECT json_build_object(
 'transfers',(SELECT count(*) FROM transfers WHERE id='$transferID'::uuid AND status='posted'),
 'journals',(SELECT count(*) FROM journal_transactions WHERE transfer_id='$transferID'::uuid),
 'postings',(SELECT count(*) FROM ledger_postings p JOIN journal_transactions j ON j.id=p.journal_transaction_id WHERE j.transfer_id='$transferID'::uuid),
 'idempotency',(SELECT count(*) FROM idempotency_requests WHERE operation='transfers.create.v1' AND idempotency_key='$IdempotencyKey' AND transfer_id='$transferID'::uuid AND state='completed')
);
"@
    Assert-LedgerSyncAcceptance ([int]$database.transfers -eq 1 -and [int]$database.journals -eq 1 -and [int]$database.postings -eq 2 -and [int]$database.idempotency -eq 1) "Lost-response transfer was not durably committed exactly once."
    $replay = Invoke-LedgerSyncAcceptanceTransferOutcome -Session $Session -CSRFToken $CSRFToken -SourceAccountID $SourceAccountID -DestinationAccountID $DestinationAccountID -AmountMinor $AmountMinor -IdempotencyKey $IdempotencyKey -ExpectReplay
    Assert-LedgerSyncAcceptance ([string]$replay.Response.Payload.transfer_id -ceq $transferID) "Lost-response retry returned a different transfer."
    $changed = Invoke-LedgerSyncAcceptanceTransferOutcome -Session $Session -CSRFToken $CSRFToken -SourceAccountID $SourceAccountID -DestinationAccountID $DestinationAccountID -AmountMinor ([string]([int64]$AmountMinor + 1)) -IdempotencyKey $IdempotencyKey -ExpectedStatus 409 -ExpectedErrorCode 'idempotency_conflict'
    $sourceAfter = Get-LedgerSyncAcceptanceBalance -Session $Session -AccountID $SourceAccountID
    $destinationAfter = Get-LedgerSyncAcceptanceBalance -Session $Session -AccountID $DestinationAccountID
    Assert-LedgerSyncAcceptance (([int64]$sourceBefore.available_minor - [int64]$sourceAfter.available_minor) -eq [int64]$AmountMinor) "Lost-response transfer debited the source other than once."
    Assert-LedgerSyncAcceptance (([int64]$destinationAfter.available_minor - [int64]$destinationBefore.available_minor) -eq [int64]$AmountMinor) "Lost-response transfer credited the destination other than once."
    foreach ($accountID in @($SourceAccountID,$DestinationAccountID)) {
        $history = Invoke-LedgerSyncAcceptanceGET -Session $Session -Path "/api/accounts/$accountID/transactions?limit=100"
        Assert-LedgerSyncAcceptance (@($history.transactions | Where-Object transfer_id -eq $transferID).Count -eq 1) "Immediate account history omitted or duplicated the lost-response transfer."
    }
    return [pscustomobject]@{ TransferID = $transferID; Key = $IdempotencyKey; Body = $body }
}

function Test-LedgerSyncAcceptanceTransferDetail {
    param([Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session,[Parameter(Mandatory = $true)][string]$TransferID,[Parameter(Mandatory = $true)][string]$AmountMinor)
    $detail = Invoke-LedgerSyncAcceptanceGET -Session $Session -Path "/api/transfers/$TransferID"
    Assert-LedgerSyncAcceptance ([string]$detail.financial_status -ceq 'posted' -and [string]$detail.amount_minor -ceq $AmountMinor -and @($detail.postings).Count -eq 2) "Transfer detail omitted exact financial evidence."
    $directions = @($detail.postings | ForEach-Object { [string]$_.direction } | Sort-Object)
    Assert-LedgerSyncAcceptance (($directions -join ',') -ceq 'credit,debit') "Transfer detail did not prove one debit and one credit."
}

function Test-LedgerSyncAcceptanceReconciliationBoundary {
    param([Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session,[Parameter(Mandatory = $true)][string]$CSRFToken,[Parameter(Mandatory = $true)][string]$IdempotencyKey)
    $first = Invoke-LedgerSyncAcceptanceJSON -Session $Session -Method POST -Path '/api/reconciliation/runs' -CSRFToken $CSRFToken -IdempotencyKey $IdempotencyKey -Body @{} -ExpectedStatus @(200,201)
    $runID = [string]$first.Payload.run_id
    Assert-LedgerSyncAcceptance ($runID -cmatch $script:LedgerSyncAcceptanceUUIDPattern -and [string]$first.Payload.status -ceq 'matched' -and [string]$first.Payload.mismatch_count -ceq '0') "Reconciliation response was not a matched exact-string result."
    $first = $null
    $database = Invoke-LedgerSyncAcceptanceSQLJSON -SQL "SELECT json_build_object('runs',(SELECT count(*) FROM reconciliation_runs WHERE id='$runID'::uuid AND status='matched' AND mismatch_count=0),'idempotency',(SELECT count(*) FROM idempotency_requests WHERE operation='reconciliation.run.v1' AND idempotency_key='$IdempotencyKey' AND state='completed'),'audit',(SELECT count(*) FROM audit_events WHERE target_id='$runID' AND event_type='reconciliation.completed'));"
    Assert-LedgerSyncAcceptance ([int]$database.runs -eq 1 -and [int]$database.idempotency -eq 1 -and [int]$database.audit -eq 1) "Reconciliation response-boundary commit was not atomic and exactly once."
    $replay = Invoke-LedgerSyncAcceptanceJSON -Session $Session -Method POST -Path '/api/reconciliation/runs' -CSRFToken $CSRFToken -IdempotencyKey $IdempotencyKey -Body @{} -ExpectedStatus @(200,201)
    Assert-LedgerSyncAcceptance ([string]$replay.Payload.run_id -ceq $runID -and [string]$replay.Headers['Idempotent-Replay'] -ieq 'true') "Reconciliation response-boundary retry did not replay its run."
    return $runID
}

function Wait-LedgerSyncAcceptanceOutbox {
    param([ValidateRange(1,120)][int]$TimeoutSeconds = 60)
    $deadline = [DateTimeOffset]::UtcNow.AddSeconds($TimeoutSeconds)
    do {
        $summary = Get-LedgerSyncOperationalSummary
        if ([int64]$summary.outbox_pending -eq 0 -and [int64]$summary.outbox_dead -eq 0) { return $summary }
        Start-Sleep -Milliseconds 500
    } while ([DateTimeOffset]::UtcNow -lt $deadline)
    throw "Acceptance outbox did not drain without dead events."
}

function Test-LedgerSyncAcceptanceOperationalEvidence {
    param([Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session,[Parameter(Mandatory = $true)][string]$TransferID)
    Wait-LedgerSyncAcceptanceOutbox | Out-Null
    $diagnostics = (Invoke-LedgerSyncAcceptanceJSON -Session $Session -Method GET -Path '/api/local/diagnostics').Payload
    Assert-LedgerSyncAcceptance ([string]$diagnostics.overall_state -ceq 'ready') "Local Status was not ready after the financial journey."
    Assert-LedgerSyncAcceptance ([string]$diagnostics.financial_authority.postgres.state -ceq 'reachable' -and [string]$diagnostics.financial_authority.latest_reconciliation.status -ceq 'matched') "Local Status did not preserve PostgreSQL and reconciliation truth."
    Assert-LedgerSyncAcceptance ([string]$diagnostics.delivery_cache.outbox.pending_count -ceq '0' -and [string]$diagnostics.delivery_cache.outbox.dead_count -ceq '0' -and [string]$diagnostics.delivery_cache.redis.label -ceq 'disposable_cache') "Local Status did not separate healthy delivery/cache evidence."
    $events = (Invoke-LedgerSyncAcceptanceJSON -Session $Session -Method GET -Path "/api/events?relatedId=$TransferID&limit=20").Payload
    $related = @($events.events | Where-Object { [string]$_.transfer_id -ceq $TransferID })
    Assert-LedgerSyncAcceptance ($related.Count -eq 2 -and @($related | Where-Object state -eq 'published').Count -eq 2) "Transfer outbox evidence was not exactly two published events."
    foreach ($event in $related) {
        $detail = (Invoke-LedgerSyncAcceptanceJSON -Session $Session -Method GET -Path "/api/events/$([string]$event.event_id)").Payload
        Assert-LedgerSyncAcceptance ([string]$detail.state -ceq 'published' -and @($detail.delivery_attempts).Count -eq 0) "Event detail manufactured or omitted delivery truth."
    }
    $explanation = (Invoke-LedgerSyncAcceptanceJSON -Session $Session -Method GET -Path "/api/transfers/$TransferID/explainability").Payload
    Assert-LedgerSyncAcceptance (@($explanation.stages).Count -eq 7) "Explainability omitted a required evidence stage."
    $kinds = @($explanation.stages | ForEach-Object { [string]$_.kind })
    Assert-LedgerSyncAcceptance (($kinds -join ',') -ceq 'request,transfer,journal_postings,balance_versions,outbox,delivery,reconciliation') "Explainability stage ordering drifted."
    Assert-LedgerSyncAcceptance ([string]$explanation.stages[0].state -ceq 'available' -and [string]$explanation.stages[4].state -ceq 'available') "Explainability omitted retained request or outbox evidence."
    Assert-LedgerSyncAcceptance ([string]$explanation.stages[5].state -ceq 'missing' -and [string]$explanation.stages[5].reason_code -ceq 'no_delivery_attempts') "Explainability confused outbox publication with downstream delivery."
}

function Test-LedgerSyncAcceptanceOrientation {
    param([Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session)
    $orientation = (Invoke-LedgerSyncAcceptanceJSON -Session $Session -Method GET -Path '/api/local/orientation').Payload
    $expected = 'inspect_account,create_account,fund_account,inspect_transfer,run_reconciliation,inspect_delivery,create_backup'
    Assert-LedgerSyncAcceptance (@($orientation.steps).Count -eq 7 -and ((@($orientation.steps | ForEach-Object { [string]$_.id }) -join ',') -ceq $expected)) "Direct orientation contract drifted."
    return $orientation
}

function ConvertFrom-LedgerSyncAcceptanceCSV {
    param([Parameter(Mandatory = $true)][string]$Content,[Parameter(Mandatory = $true)][string[]]$ExpectedHeader)
    Assert-LedgerSyncAcceptance ([Text.Encoding]::UTF8.GetByteCount($Content) -le 16MB) "Acceptance CSV exceeded the browser response ceiling."
    $lines = @($Content -split "`r?`n" | Where-Object { $_.Length -gt 0 })
    Assert-LedgerSyncAcceptance ($lines.Count -ge 2 -and $lines.Count -le 10001) "Acceptance CSV did not contain a bounded header and evidence row."
    foreach ($line in $lines) {
        Assert-LedgerSyncAcceptance ($line -cmatch '^"(?:[^"]|"")*"(?:,"(?:[^"]|"")*")*$') "Acceptance CSV contained an unquoted field."
    }
    $rows = @($Content | ConvertFrom-Csv)
    Assert-LedgerSyncAcceptance ((@($rows[0].PSObject.Properties.Name) -join ',') -ceq ($ExpectedHeader -join ',')) "Acceptance CSV header drifted."
    Assert-LedgerSyncAcceptance (@($rows | Where-Object { [string]$_.schema_version -cne '1' }).Count -eq 0) "Acceptance CSV schema version drifted."
    return $rows
}

function Get-LedgerSyncAcceptanceCSV {
    param([Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session,[Parameter(Mandatory = $true)][string]$Path,[Parameter(Mandatory = $true)][string]$Family,[Parameter(Mandatory = $true)][string[]]$ExpectedHeader)
    $response = Invoke-WebRequest -UseBasicParsing -WebSession $Session -TimeoutSec 20 -Uri "$script:LedgerSyncWebUrl$Path" -Headers @{ Accept = 'text/csv' }
    Assert-LedgerSyncAcceptance ([int]$response.StatusCode -eq 200 -and [string]$response.Headers['Cache-Control'] -match 'no-store') "Acceptance CSV response was not a no-store success."
    Assert-LedgerSyncAcceptance ([string]$response.Headers['Content-Type'] -match '^text/csv' -and [string]$response.Headers['X-LedgerSync-Export-Schema'] -ceq '1') "Acceptance CSV response headers drifted."
    $filenamePattern = '^attachment; filename="ledgersync-{0}-\d{{8}}T\d{{6}}Z-v1\.csv"$' -f [regex]::Escape($Family)
    Assert-LedgerSyncAcceptance ([string]$response.Headers['Content-Disposition'] -cmatch $filenamePattern) "Acceptance CSV filename was not canonical."
    return @(ConvertFrom-LedgerSyncAcceptanceCSV -Content ([string]$response.Content) -ExpectedHeader $ExpectedHeader)
}

function Test-LedgerSyncAcceptanceExports {
    param([Parameter(Mandatory = $true)][Microsoft.PowerShell.Commands.WebRequestSession]$Session,[Parameter(Mandatory = $true)][string]$AccountID,[Parameter(Mandatory = $true)][string]$TransferID,[Parameter(Mandatory = $true)][string]$RunID,[Parameter(Mandatory = $true)][string]$AmountMinor)
    $transfers = @(Get-LedgerSyncAcceptanceCSV -Session $Session -Path "/api/exports/transfers.csv?accountId=$AccountID&status=posted&limit=100" -Family 'transfers' -ExpectedHeader @('schema_version','transfer_id','source_account_id','destination_account_id','amount_minor','currency','financial_status','delivery_status','created_at_utc','completed_at_utc','journal_transaction_id','rejection_code'))
    $transfer = @($transfers | Where-Object transfer_id -eq $TransferID)
    Assert-LedgerSyncAcceptance ($transfer.Count -eq 1 -and [string]$transfer[0].amount_minor -ceq $AmountMinor -and [string]$transfer[0].currency -ceq 'INR' -and [string]$transfer[0].financial_status -ceq 'posted') "Transfer CSV omitted exact financial evidence."
    $ledger = @(Get-LedgerSyncAcceptanceCSV -Session $Session -Path "/api/exports/accounts/$AccountID/transactions.csv?limit=100" -Family 'account-ledger' -ExpectedHeader @('schema_version','transfer_id','direction','amount_minor','currency','status','occurred_at_utc'))
    Assert-LedgerSyncAcceptance (@($ledger | Where-Object transfer_id -eq $TransferID).Count -eq 1) "Account ledger CSV omitted or duplicated the target transfer."
    $reconciliations = @(Get-LedgerSyncAcceptanceCSV -Session $Session -Path "/api/exports/reconciliation.csv?runId=$RunID&status=matched&limit=100" -Family 'reconciliation' -ExpectedHeader @('schema_version','record_type','run_id','status','correlation_id','scope','ledger_watermark','application_version','database_schema_version','checked_account_count','posting_count','mismatch_count','started_at_utc','completed_at_utc','mismatch_id','account_id','classification','currency','expected_minor','observed_minor','observed_available_minor','balance_version','created_at_utc'))
    $run = @($reconciliations | Where-Object { [string]$_.record_type -ceq 'run' -and [string]$_.run_id -ceq $RunID })
    Assert-LedgerSyncAcceptance ($run.Count -eq 1 -and [string]$run[0].status -ceq 'matched' -and [string]$run[0].mismatch_count -ceq '0') "Reconciliation CSV omitted its matched zero-mismatch run."
}

function Assert-LedgerSyncAcceptanceRuntimeHardening {
    param([Parameter(Mandatory = $true)][string]$Project,[Parameter(Mandatory = $true)][string]$RuntimeEnvironmentFile)
    Assert-LedgerSyncAcceptance (Test-LedgerSyncRuntimeEnvironmentFile -Path $RuntimeEnvironmentFile) "Acceptance runtime credential file was incomplete."
    if ($env:OS -eq 'Windows_NT') {
        Assert-LedgerSyncAcceptance ((Get-Acl -LiteralPath $RuntimeEnvironmentFile).AreAccessRulesProtected) "Acceptance runtime credential ACL inherited host permissions."
    }
    $rows = @(Get-LedgerSyncComposeRows)
    $services = @('postgres','redis','api','outbox-worker','web','migrate','demo-seed')
    foreach ($service in $services) {
        $row = @(Get-LedgerSyncServiceRow -Service $service -Rows $rows)
        Assert-LedgerSyncAcceptance ($row.Count -eq 1) "Acceptance hardening evidence omitted a required service."
        $raw = @(& docker inspect ([string]$row[0].ID) 2>$null)
        Assert-LedgerSyncAcceptance ($LASTEXITCODE -eq 0) "Acceptance service inspection failed."
        $inspect = @($raw | ConvertFrom-Json)[0]
        Assert-LedgerSyncAcceptance ($inspect.HostConfig.ReadonlyRootfs -eq $true -and $inspect.HostConfig.Privileged -eq $false) "Acceptance service filesystem/privilege hardening drifted."
        Assert-LedgerSyncAcceptance (@($inspect.HostConfig.CapDrop) -contains 'ALL' -and @($inspect.HostConfig.SecurityOpt) -contains 'no-new-privileges:true') "Acceptance service capability hardening drifted."
        Assert-LedgerSyncAcceptance (-not [string]::IsNullOrWhiteSpace([string]$inspect.Config.User)) "Acceptance service lacks an explicit non-root user."
        if ($service -eq 'web') {
            $binding = @($inspect.HostConfig.PortBindings.'3000/tcp')
            Assert-LedgerSyncAcceptance ($binding.Count -eq 1 -and [string]$binding[0].HostIp -ceq '127.0.0.1' -and [string]$binding[0].HostPort -ceq '3000') "Acceptance web port is not exact loopback 3000."
        } else {
            $published = @($inspect.HostConfig.PortBindings.PSObject.Properties)
            Assert-LedgerSyncAcceptance ($published.Count -eq 0) "A private acceptance service published a host port."
        }
    }
    $privateRaw = @(& docker network inspect "${Project}_private" 2>$null)
    Assert-LedgerSyncAcceptance ($LASTEXITCODE -eq 0) "Acceptance private network could not be inspected."
    $private = @($privateRaw | ConvertFrom-Json)[0]
    Assert-LedgerSyncAcceptance ($private.Internal -eq $true) "Acceptance private network is externally routable."
}

function Get-LedgerSyncAcceptanceNormalState {
    $summary = Get-LedgerSyncOperationalSummary
    $volumes = @(& docker volume ls --filter "label=com.docker.compose.project=$script:LedgerSyncComposeProject" --format '{{.Name}}' 2>$null | Sort-Object)
    Assert-LedgerSyncAcceptance ($LASTEXITCODE -eq 0 -and $volumes.Count -eq 2) "Normal state did not contain its exact two named volumes."
    return [pscustomobject]@{ Financial = (Get-LedgerSyncFinancialFingerprint); Operational = $summary; Volumes = $volumes }
}

function Compare-LedgerSyncAcceptanceNormalState {
    param([Parameter(Mandatory = $true)][object]$Before,[Parameter(Mandatory = $true)][object]$After)
    Compare-LedgerSyncFinancialFingerprint -Before $Before.Financial -After $After.Financial
    foreach ($field in @('migration_version','migration_count','outbox_pending','outbox_dead','reconciliation_status','reconciliation_mismatches')) {
        Assert-LedgerSyncAcceptance ([string]$Before.Operational.$field -ceq [string]$After.Operational.$field) "Normal operational state changed at $field."
    }
    Assert-LedgerSyncAcceptance ((@($Before.Volumes) -join '|') -ceq (@($After.Volumes) -join '|')) "Normal named volume identity changed."
}

function Assert-LedgerSyncAcceptanceResourcesAbsent {
    param([Parameter(Mandatory = $true)][string]$Project)
    Assert-LedgerSyncAcceptance ($Project -cmatch $script:LedgerSyncAcceptanceProjectPattern) "Acceptance resource cleanup refused an invalid project."
    $containers = @(& docker ps -a --filter "label=com.docker.compose.project=$Project" --format '{{.ID}}' 2>$null)
    $volumes = @(& docker volume ls --filter "label=com.docker.compose.project=$Project" --format '{{.Name}}' 2>$null)
    $networks = @(& docker network ls --filter "label=com.docker.compose.project=$Project" --format '{{.Name}}' 2>$null)
    Assert-LedgerSyncAcceptance ($LASTEXITCODE -eq 0 -and $containers.Count -eq 0 -and $volumes.Count -eq 0 -and $networks.Count -eq 0) "Acceptance cleanup left exact project resources behind."
}
