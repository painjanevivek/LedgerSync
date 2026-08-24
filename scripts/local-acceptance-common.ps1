Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

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
